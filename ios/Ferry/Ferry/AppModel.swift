import Foundation
import Observation
import UIKit

@MainActor @Observable
final class AppModel {
    enum Phase: Equatable { case setup, connecting, connected, offline }

    var serverAddress: String {
        didSet {
            guard serverAddress != oldValue else { return }
            accessPassword = ""
            // A response belongs to the address that started it. If the user
            // edits that address while connecting, the old request may finish,
            // but it must no longer be allowed to replace the new input.
            if phase == .connecting, serverAddress != endpoint?.origin {
                generation = UUID()
                endpoint = nil
                token = nil
                currentDevice = nil
                phase = .setup
            }
        }
    }
    var deviceName: String
    var accessPassword = ""
    var draft = "" { didSet { draftRevision &+= 1 } }
    private(set) var phase: Phase = .setup
    private(set) var currentDevice: Device?
    private(set) var messages: [MessagePayload] = []
    private(set) var selectedFile: SelectedFile?
    private(set) var statusMessage: String?
    private(set) var sendError: String?
    private(set) var credentialWarning: String?
    private(set) var isSending = false

    private let client: any FerryServicing
    private let credentials: CredentialStoring
    private let defaults: UserDefaults
    private var endpoint: ServerEndpoint?
    private var token: String?
    private var cursor: Int64 = 0
    private var generation = UUID()
    private var pollTask: Task<Void, Never>?
    private var sendTask: Task<MessagePayload, Error>?
    private var draftRevision: UInt64 = 0
    private var fileRevision: UInt64 = 0
    private var isActive = true
    private static let serverKey = "ferry.server.origin"

    init(client: any FerryServicing = FerryClient(), credentials: CredentialStoring = KeychainCredentialStore(),
         defaults: UserDefaults = .standard) {
        self.client = client
        self.credentials = credentials
        self.defaults = defaults
        serverAddress = defaults.string(forKey: Self.serverKey) ?? "http://127.0.0.1:8080"
        deviceName = UIDevice.current.name
    }

    var canSend: Bool {
        !isSending && (selectedFile != nil || !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
    }

    func start() async {
        guard phase == .setup, let endpoint = try? ServerEndpoint(serverAddress) else { return }
        do {
            guard let stored = try credentials.token(for: endpoint.origin) else { return }
            let current = resetSession(endpoint: endpoint, token: stored)
            phase = .connecting
            await authenticate(generation: current)
        } catch {
            statusMessage = error.localizedDescription
        }
    }

    func connect() async {
        let current: UUID
        let endpoint: ServerEndpoint
        do {
            endpoint = try ServerEndpoint(serverAddress)
            current = resetSession(endpoint: endpoint, token: nil)
        } catch { statusMessage = error.localizedDescription; return }
        phase = .connecting
        statusMessage = nil
        do {
            let claim = try await client.join(endpoint: endpoint, password: accessPassword, name: deviceName)
            guard current == generation else { return }
            token = claim.token
            currentDevice = claim.device
            defaults.set(endpoint.origin, forKey: Self.serverKey)
            serverAddress = endpoint.origin
            do { try credentials.save(token: claim.token, for: endpoint.origin) }
            catch { credentialWarning = error.localizedDescription }
            accessPassword = ""
            phase = .connected
            await refreshMessages(generation: current)
            startPolling()
        } catch { handle(error, generation: current) }
    }

    func send() async {
        guard let endpoint, let token, canSend else { return }
        let current = generation
        let sentFile = selectedFile
        let sentDraft = draft
        let sentDraftRevision = draftRevision
        let sentFileRevision = fileRevision
        isSending = true
        sendError = nil
        let task = Task { [client] in
            if let file = sentFile { return try await client.sendFile(endpoint: endpoint, token: token, file: file) }
            return try await client.sendText(endpoint: endpoint, token: token, text: sentDraft)
        }
        sendTask = task
        defer {
            if current == generation {
                sendTask = nil
                isSending = false
            }
        }
        do {
            let message = try await task.value
            guard current == generation else { return }
            append(message)
            if sentFile != nil {
                if sentFileRevision == fileRevision { setSelectedFile(nil) }
            } else if sentDraftRevision == draftRevision {
                draft = ""
            }
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch {
            guard current == generation else { return }
            if error as? FerryClient.ClientError == .unauthorized {
                handle(error, generation: current)
            } else {
                sendError = error.localizedDescription
                if error is URLError || error as? FerryClient.ClientError == .invalidResponse { phase = .offline }
            }
        }
    }

    func selectFile(_ url: URL) {
        let accessing = url.startAccessingSecurityScopedResource()
        defer { if accessing { url.stopAccessingSecurityScopedResource() } }
        do {
            let values = try url.resourceValues(forKeys: [.fileSizeKey, .nameKey])
            let size = Int64(values.fileSize ?? 0)
            guard size <= SelectedFile.maximumBytes else { throw FerryClient.ClientError.rejected("Files must be 64 MB or smaller.") }
            setSelectedFile(SelectedFile(url: url, name: values.name ?? url.lastPathComponent, size: size))
            sendError = nil
        } catch { sendError = error.localizedDescription }
    }

    func clearSelectedFile() { if selectedFile != nil { setSelectedFile(nil) } }

    func fileImportFailed(_ error: Error) { sendError = error.localizedDescription }

    func disconnect() {
        guard let endpoint else { phase = .setup; return }
        let removalError: String?
        do {
            try credentials.removeToken(for: endpoint.origin)
            removalError = nil
        } catch {
            removalError = error.localizedDescription
        }
        _ = resetSession(endpoint: endpoint, token: nil)
        phase = .setup
        statusMessage = removalError
    }

    func setActive(_ active: Bool) {
        isActive = active
        if active {
            if phase == .connected || phase == .offline { startPolling() }
            else { Task { await start() } }
        } else {
            pollTask?.cancel()
            pollTask = nil
            sendTask?.cancel()
        }
    }

    private func resetSession(endpoint: ServerEndpoint, token: String?) -> UUID {
        pollTask?.cancel()
        pollTask = nil
        sendTask?.cancel()
        sendTask = nil
        generation = UUID()
        self.endpoint = endpoint
        self.token = token
        currentDevice = nil
        messages = []
        cursor = 0
        draft = ""
        isSending = false
        setSelectedFile(nil)
        sendError = nil
        credentialWarning = nil
        return generation
    }

    private func authenticate(generation current: UUID) async {
        guard let endpoint, let token else { return }
        do {
            let device = try await client.currentDevice(endpoint: endpoint, token: token)
            guard current == generation else { return }
            currentDevice = device
            phase = .connected
            statusMessage = nil
            await refreshMessages(generation: current)
            startPolling()
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch { handle(error, generation: current) }
    }

    private func startPolling() {
        guard isActive, pollTask == nil, token != nil else { return }
        let current = generation
        pollTask = Task { [weak self] in
            while !Task.isCancelled {
                await self?.refreshMessages(generation: current)
                try? await Task.sleep(for: .seconds(2))
            }
        }
    }

    private func refreshMessages(generation current: UUID) async {
        guard current == generation, let endpoint, let token else { return }
        do {
            let page = try await client.messages(endpoint: endpoint, token: token, after: cursor)
            guard current == generation else { return }
            page.messages.forEach(append)
            cursor = page.nextCursor
            phase = .connected
            statusMessage = nil
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch { handle(error, generation: current) }
    }

    private func append(_ message: MessagePayload) {
        guard !messages.contains(where: { $0.id == message.id }) else { return }
        messages.append(message)
        messages.sort { $0.sequence < $1.sequence }
        cursor = max(cursor, message.sequence)
    }

    private func setSelectedFile(_ file: SelectedFile?) {
        fileRevision &+= 1
        selectedFile = file
    }

    private func handle(_ error: Error, generation current: UUID) {
        guard current == generation else { return }
        statusMessage = error.localizedDescription
        if error as? FerryClient.ClientError == .unauthorized {
            guard let endpoint else { phase = .setup; return }
            try? credentials.removeToken(for: endpoint.origin)
            _ = resetSession(endpoint: endpoint, token: nil)
            phase = .setup
        } else {
            phase = token == nil ? .setup : .offline
        }
    }
}
