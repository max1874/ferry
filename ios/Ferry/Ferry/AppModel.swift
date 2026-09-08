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
    var clipboardSyncEnabled: Bool {
        didSet {
            guard clipboardSyncEnabled != oldValue else { return }
            defaults.set(clipboardSyncEnabled, forKey: Self.clipboardSyncKey)
            clipboardStatus = nil
        }
    }
    private(set) var clipboardStatus: String?
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
    private let pasteboard: any PasteboardWriting
    private let attachments: any AttachmentLoading
    // What Ferry last put on this pasteboard. An incoming message that already
    // matches it is not written again, which is what stops two devices from
    // handing the same text back and forth.
    private var clipboardEcho: String?
    // The first page after connecting is everything that happened before the
    // app was running. Writing it would replace what the user was carrying with
    // an entry they never asked for, so it only sets the baseline.
    private var clipboardPrimed = false
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
    private static let clipboardSyncKey = "ferry.clipboard.sync"

    init(client: any FerryServicing = FerryClient(), credentials: CredentialStoring = KeychainCredentialStore(),
         defaults: UserDefaults = .standard, pasteboard: any PasteboardWriting = SystemPasteboard(),
         attachments: any AttachmentLoading = FerryClient()) {
        self.client = client
        self.credentials = credentials
        self.defaults = defaults
        self.pasteboard = pasteboard
        self.attachments = attachments
        serverAddress = defaults.string(forKey: Self.serverKey) ?? "http://127.0.0.1:8080"
        deviceName = UIDevice.current.name
        clipboardSyncEnabled = defaults.bool(forKey: Self.clipboardSyncKey)
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
        // The next page is a full backfill again, so it must not reach the
        // pasteboard, and a new session's echo is nobody's.
        clipboardPrimed = false
        clipboardEcho = nil
        clipboardStatus = nil
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
            await syncClipboard(from: page.messages, generation: current)
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch { handle(error, generation: current) }
    }

    /// Writes the newest message another device sent onto this pasteboard.
    ///
    /// Only the newest one: coming back to an app that missed twenty messages
    /// must leave one pasteboard entry, not replay twenty. Only in the
    /// foreground, because replacing the pasteboard of an app the user is
    /// actually working in is not Ferry's to do.
    private func syncClipboard(from page: [MessagePayload], generation current: UUID) async {
        guard clipboardSyncEnabled, isActive, current == generation else { return }
        guard clipboardPrimed else {
            clipboardPrimed = true
            return
        }
        guard let latest = page.filter({ !$0.isCurrentDevice }).max(by: { $0.sequence < $1.sequence }) else { return }
        switch latest.kind {
        case .text:
            guard let text = latest.text, text != clipboardEcho else { return }
            pasteboard.write(text: text)
            clipboardEcho = text
            clipboardStatus = "Copied the newest message."
        case .file:
            await syncImage(latest, generation: current)
        }
    }

    private func syncImage(_ message: MessagePayload, generation current: UUID) async {
        guard let file = message.file, file.mediaType.hasPrefix("image/"),
              let endpoint, let token else { return }
        do {
            let data = try await attachments.attachment(endpoint: endpoint, token: token, path: file.downloadURL)
            // The download outlives nothing: a revoked session or a backgrounded
            // app must not have its pasteboard written by a reply that arrived late.
            guard current == generation, isActive, clipboardSyncEnabled else { return }
            guard pasteboard.write(imageData: data) else {
                clipboardStatus = "\(file.name) is not an image this device can paste."
                return
            }
            clipboardEcho = nil
            clipboardStatus = "Copied \(file.name)."
        } catch is CancellationError {
            return
        } catch let error as URLError where error.code == .cancelled {
            return
        } catch {
            guard current == generation else { return }
            clipboardStatus = "Could not copy \(file.name): \(error.localizedDescription)"
        }
    }

    /// Sends what a system paste control handed over. iOS never lets Ferry read
    /// the pasteboard on its own, so this only ever runs from the user's tap.
    func sendPasted(text: String) async {
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            clipboardStatus = "That paste had no text to send."
            return
        }
        clipboardStatus = nil
        clearSelectedFile()
        draft = text
        await send()
        clipboardEcho = text
    }

    func sendPasted(imageData: Data, name: String) async {
        clipboardStatus = nil
        do {
            let url = FileManager.default.temporaryDirectory.appendingPathComponent(name)
            try imageData.write(to: url, options: .atomic)
            defer { try? FileManager.default.removeItem(at: url) }
            // Reuse the ordinary attachment path so the pasted image inherits
            // the same size limit, cancellation and failure reporting.
            selectFile(url)
            // selectFile reports its own refusal through sendError. Sending
            // anyway would quietly post the text draft in the image's place.
            guard selectedFile != nil else { return }
            await send()
            clipboardEcho = nil
        } catch {
            clipboardStatus = "Could not send the pasted image: \(error.localizedDescription)"
        }
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
