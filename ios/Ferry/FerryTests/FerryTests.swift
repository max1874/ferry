import XCTest
@testable import Ferry

@MainActor
final class FerryTests: XCTestCase {
    func testServerEndpointNormalizesOriginAndRejectsPathsOrCredentials() throws {
        XCTAssertEqual(try ServerEndpoint(" HTTPS://Example.COM:443/ ").origin, "https://example.com")
        XCTAssertEqual(try ServerEndpoint("http://EXAMPLE.com:80").origin, "http://example.com")
        XCTAssertThrowsError(try ServerEndpoint("ftp://example.com"))
        XCTAssertThrowsError(try ServerEndpoint("http://user:secret@example.com"))
        XCTAssertThrowsError(try ServerEndpoint("http://example.com/admin"))
    }

    func testChangingServerDoesNotSendPreviousServersPassword() async {
        let service = ScriptedService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "old server secret"

        model.serverAddress = "http://new.example"
        await model.connect()

        XCTAssertEqual(service.lastPassword, "")
        XCTAssertEqual(model.serverAddress, "http://new.example")
        model.setActive(false)
    }

    func testChangingServerDuringJoinInvalidatesOldResponse() async throws {
        let service = DelayedJoinService()
        let model = makeModel(client: service, credentials: MemoryCredentials())

        let oldConnection = Task { await model.connect() }
        await waitUntil { service.isWaiting }
        model.serverAddress = "http://new.example"
        service.finishJoin()
        await oldConnection.value

        XCTAssertEqual(model.serverAddress, "http://new.example")
        XCTAssertEqual(model.phase, .setup)
        XCTAssertNil(model.currentDevice)
        model.setActive(false)
    }

    func testMessageDecoderRejectsUnknownAndMismatchedKinds() throws {
        XCTAssertThrowsError(try decodeMessage(kind: "video", text: "hello", file: "null"))
        XCTAssertThrowsError(try decodeMessage(kind: "text", text: "hello", file: fileJSON))
        XCTAssertThrowsError(try decodeMessage(kind: "file", text: "hello", file: fileJSON))
        XCTAssertEqual(try decodeMessage(kind: "text", text: "hello", file: "null").text, "hello")
    }

    func testClientBuildsAuthenticatedRequestAndMapsUnauthorized() async throws {
        let session = URLSession(configuration: configuration())
        let client = FerryClient(session: session)
        URLStub.handler = { request in
            XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:8080/api/v1/messages?after=7&limit=200")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer device-token")
            return (HTTPURLResponse(url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!, Data())
        }
        do {
            let _: MessagesPage = try await client.messages(endpoint: ServerEndpoint("http://127.0.0.1:8080"), token: "device-token", after: 7)
            XCTFail("Expected unauthorized")
        } catch {
            XCTAssertEqual(error as? FerryClient.ClientError, .unauthorized)
        }
    }

    func testClientSendsPasswordAndPreservesJoinRejection() async throws {
        let session = URLSession(configuration: configuration())
        let client = FerryClient(session: session)
        URLStub.handler = { request in
            XCTAssertEqual(request.url?.path, "/api/v1/access/join")
            XCTAssertNil(request.value(forHTTPHeaderField: "Authorization"))
            XCTAssertEqual(request.value(forHTTPHeaderField: "X-Ferry-Device-Kind"), "iphone")
            let stream = try XCTUnwrap(request.httpBodyStream)
            stream.open()
            defer { stream.close() }
            var requestData = Data()
            var buffer = [UInt8](repeating: 0, count: 1024)
            while stream.hasBytesAvailable {
                let count = stream.read(&buffer, maxLength: buffer.count)
                if count <= 0 { break }
                requestData.append(buffer, count: count)
            }
            let payload = try JSONSerialization.jsonObject(with: requestData) as? [String: String]
            XCTAssertEqual(payload, ["device_name": "Test iPhone", "password": "right horse"])
            let body = Data("""
            {"error":{"code":"invalid_password","message":"password is incorrect"}}
            """.utf8)
            return (HTTPURLResponse(url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!, body)
        }

        do {
            let _: AccessClaim = try await client.join(
                endpoint: ServerEndpoint("http://127.0.0.1:8080"), password: "right horse", name: "Test iPhone"
            )
            XCTFail("Expected access rejection")
        } catch {
            XCTAssertEqual(error as? FerryClient.ClientError, .rejected("password is incorrect"))
        }
    }

    func testPollingRestartsAfterRevocationAndReconnect() async throws {
        let service = ScriptedService()
        let credentials = MemoryCredentials()
        let suite = "FerryTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let model = AppModel(client: service, credentials: credentials, defaults: defaults)
        model.serverAddress = "http://127.0.0.1:8080"
        model.deviceName = "Test iPhone"
        model.accessPassword = "1111"

        await model.connect()
        await waitUntil { model.phase == .setup }
        XCTAssertNil(credentials.token)

        model.accessPassword = "2222"
        await model.connect()
        await waitUntil { service.messageCalls >= 4 }
        XCTAssertEqual(model.phase, .connected)
        XCTAssertGreaterThanOrEqual(service.messageCalls, 4, "A reconnected session must own a fresh polling task")
        model.setActive(false)
    }

    func testStaleAuthenticationCannotReplaceNewConnection() async throws {
        let service = DelayedAuthenticationService()
        let credentials = MemoryCredentials()
        credentials.token = "old-token"
        let suite = "FerryTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defaults.set("http://old.example", forKey: "ferry.server.origin")
        defer { defaults.removePersistentDomain(forName: suite) }
        let model = AppModel(client: service, credentials: credentials, defaults: defaults)

        let oldAuthentication = Task { await model.start() }
        await waitUntil { service.isWaiting }
        model.serverAddress = "http://new.example"
        model.deviceName = "New iPhone"
        model.accessPassword = "3333"
        await model.connect()
        service.finishOldAuthentication()
        await oldAuthentication.value

        XCTAssertEqual(model.currentDevice?.id, "new")
        XCTAssertEqual(model.serverAddress, "http://new.example")
        XCTAssertEqual(model.phase, .connected)
        model.setActive(false)
    }

    func testBackgroundCancellationDoesNotTurnConnectedSessionOffline() async throws {
        let service = CancellablePollingService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        await waitUntil { service.isPolling }

        model.setActive(false)
        await waitUntil { !service.isPolling }
        XCTAssertEqual(model.phase, .connected)
        XCTAssertNil(model.statusMessage)
    }

    func testCredentialSaveFailureRemainsVisibleWhileConnected() async throws {
        let credentials = MemoryCredentials()
        credentials.saveError = TestError.keychain
        let model = makeModel(client: FileSendingService(), credentials: credentials)
        model.accessPassword = "4444"
        await model.connect()

        XCTAssertEqual(model.phase, .connected)
        XCTAssertEqual(model.credentialWarning, "Test Keychain failure")
        model.setActive(false)
    }

    func testSendingFilePreservesExistingTextDraft() async throws {
        let service = FileSendingService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        let fileURL = FileManager.default.temporaryDirectory.appending(path: "ferry-\(UUID().uuidString).txt")
        try Data("fixture".utf8).write(to: fileURL)
        defer { try? FileManager.default.removeItem(at: fileURL) }
        model.draft = "keep this draft"
        model.selectFile(fileURL)

        await model.send()

        XCTAssertEqual(model.draft, "keep this draft")
        XCTAssertNil(model.selectedFile)
        XCTAssertEqual(model.messages.last?.file?.name, fileURL.lastPathComponent)
        model.setActive(false)
    }

    func testOversizedFileIsRejectedBeforeUpload() throws {
        let model = makeModel(client: FileSendingService(), credentials: MemoryCredentials())
        let fileURL = FileManager.default.temporaryDirectory.appending(path: "oversized-\(UUID().uuidString).bin")
        FileManager.default.createFile(atPath: fileURL.path, contents: nil)
        let handle = try FileHandle(forWritingTo: fileURL)
        try handle.truncate(atOffset: UInt64(SelectedFile.maximumBytes + 1))
        try handle.close()
        defer { try? FileManager.default.removeItem(at: fileURL) }

        model.selectFile(fileURL)

        XCTAssertNil(model.selectedFile)
        XCTAssertEqual(model.sendError, "Files must be 64 MB or smaller.")
    }

    func testSendFailureSurvivesAHealthyMessagePoll() async throws {
        let service = FailedSendService(error: FerryClient.ClientError.rejected("Upload rejected"))
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        model.draft = "keep me"

        await model.send()
        model.setActive(false)
        model.setActive(true)
        await waitUntil { service.messageCalls >= 2 }

        XCTAssertEqual(model.phase, .connected)
        XCTAssertEqual(model.sendError, "Upload rejected")
        XCTAssertEqual(model.draft, "keep me")
        model.setActive(false)
    }

    func testSlowTextSendDoesNotClearDraftEditedAwayAndBackToSameValue() async throws {
        let service = DelayedSendService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        model.draft = "first"
        let send = Task { await model.send() }
        await waitUntil { service.isSendingText }

        model.draft = "temporary"
        model.draft = "first"
        service.completeText()
        await send.value

        XCTAssertEqual(model.draft, "first")
        XCTAssertEqual(model.messages.last?.text, "first")
        model.setActive(false)
    }

    func testSelectingAFileDuringSlowTextSendDoesNotRetainSentDraft() async throws {
        let service = DelayedSendService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        model.draft = "first"
        let send = Task { await model.send() }
        await waitUntil { service.isSendingText }
        let file = FileManager.default.temporaryDirectory.appending(path: "cross-field-\(UUID().uuidString).txt")
        try Data("file".utf8).write(to: file)
        defer { try? FileManager.default.removeItem(at: file) }

        model.selectFile(file)
        service.completeText()
        await send.value

        XCTAssertEqual(model.draft, "")
        XCTAssertEqual(model.selectedFile?.url, file)
        model.setActive(false)
    }

    func testSlowFileSendDoesNotClearSameFileReselectedAfterRemoval() async throws {
        let service = DelayedSendService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        let first = FileManager.default.temporaryDirectory.appending(path: "first-\(UUID().uuidString).txt")
        try Data("first".utf8).write(to: first)
        defer {
            try? FileManager.default.removeItem(at: first)
        }
        model.selectFile(first)
        let send = Task { await model.send() }
        await waitUntil { service.isSendingFile }

        model.clearSelectedFile()
        model.selectFile(first)
        service.completeFile()
        await send.value

        XCTAssertEqual(model.selectedFile?.url, first)
        XCTAssertEqual(model.messages.last?.file?.name, first.lastPathComponent)
        model.setActive(false)
    }

    func testOfflineUserCanDisconnectAndReturnToSetup() async throws {
        let credentials = MemoryCredentials()
        let service = FailedSendService(error: URLError(.cannotConnectToHost))
        let model = makeModel(client: service, credentials: credentials)
        model.accessPassword = "4444"
        await model.connect()
        model.draft = "old server draft"

        await model.send()
        XCTAssertEqual(model.phase, .offline)
        model.disconnect()

        XCTAssertEqual(model.phase, .setup)
        XCTAssertEqual(model.serverAddress, "http://127.0.0.1:8080")
        XCTAssertNil(credentials.token)
        XCTAssertEqual(model.draft, "")
        XCTAssertNil(model.sendError)
    }

    func testKeychainRoundTripIsOriginScoped() throws {
        let store = KeychainCredentialStore()
        let first = "http://keychain-\(UUID().uuidString).test"
        let second = "http://keychain-\(UUID().uuidString).test"
        defer {
            try? store.removeToken(for: first)
            try? store.removeToken(for: second)
        }

        try store.save(token: "first-token", for: first)

        XCTAssertEqual(try store.token(for: first), "first-token")
        XCTAssertNil(try store.token(for: second))
        try store.removeToken(for: first)
        XCTAssertNil(try store.token(for: first))
    }

    func testClientRechecksActualFileSizeWhenMetadataIsWrong() async throws {
        let fileURL = FileManager.default.temporaryDirectory.appending(path: "bypass-\(UUID().uuidString).bin")
        FileManager.default.createFile(atPath: fileURL.path, contents: nil)
        let handle = try FileHandle(forWritingTo: fileURL)
        try handle.truncate(atOffset: UInt64(SelectedFile.maximumBytes + 1))
        try handle.close()
        defer { try? FileManager.default.removeItem(at: fileURL) }
        let client = FerryClient(session: URLSession(configuration: configuration()))
        URLStub.handler = { request in
            XCTFail("Oversized bytes reached the network")
            return (HTTPURLResponse(url: request.url!, statusCode: 500, httpVersion: nil, headerFields: nil)!, Data())
        }

        do {
            let _: MessagePayload = try await client.sendFile(
                endpoint: ServerEndpoint("http://127.0.0.1:8080"), token: "token",
                file: SelectedFile(url: fileURL, name: "bypass.bin", size: 0)
            )
            XCTFail("Expected the actual byte count to be rejected")
        } catch {
            XCTAssertEqual(error as? FerryClient.ClientError, .rejected("Files must be 64 MB or smaller."))
        }
    }

    func testRevocationClearsOldDraftAndInFlightSendState() async throws {
        let service = RevokingDuringSendService()
        let model = makeModel(client: service, credentials: MemoryCredentials())
        model.accessPassword = "4444"
        await model.connect()
        await waitUntil { service.isPolling }
        model.draft = "secret from old identity"
        let send = Task { await model.send() }
        await waitUntil { service.isSending }

        service.revoke()
        await waitUntil { model.phase == .setup }

        XCTAssertEqual(model.draft, "")
        XCTAssertFalse(model.isSending)
        await send.value
        XCTAssertTrue(service.sendWasCancelled)
        XCTAssertTrue(model.messages.isEmpty)
    }

    func testAnImageAttachmentIsFetchedOnceAndKeptForRedraws() async throws {
        let service = AttachmentService(data: Self.onePixelPNG)
        let model = makeModel(client: service, credentials: MemoryCredentials())
        await model.connect()
        let message = try decodeImageMessage()

        await model.loadImage(for: message)
        XCTAssertNotNil(model.images[message.id] ?? nil)
        XCTAssertEqual(service.attachmentCalls, 1)

        // A redraw asks again; a message that already resolved must not refetch.
        await model.loadImage(for: message)
        XCTAssertEqual(service.attachmentCalls, 1)
        model.setActive(false)
    }

    func testAFailedAttachmentIsRecordedSoTheRowFallsBackInsteadOfRetrying() async throws {
        let service = AttachmentService(data: nil)
        let model = makeModel(client: service, credentials: MemoryCredentials())
        await model.connect()
        let message = try decodeImageMessage()

        await model.loadImage(for: message)
        let entry = try XCTUnwrap(model.images[message.id])
        XCTAssertNil(entry)
        XCTAssertEqual(service.attachmentCalls, 1)

        await model.loadImage(for: message)
        XCTAssertEqual(service.attachmentCalls, 1)
        model.setActive(false)
    }

    func testAnAttachmentArrivingAfterDisconnectDoesNotEnterTheNewTimeline() async throws {
        let service = SuspendingAttachmentService(data: Self.onePixelPNG)
        let model = makeModel(client: service, credentials: MemoryCredentials())
        await model.connect()
        let message = try decodeImageMessage()

        let load = Task { await model.loadImage(for: message) }
        await waitUntil { service.isLoading }
        model.disconnect()
        service.release()
        await load.value

        XCTAssertTrue(model.images.isEmpty)
    }

    /// A file message carries no text at all; the decoder rejects a payload
    /// that has both, so this cannot go through `decodeMessage`.
    private func decodeImageMessage() throws -> MessagePayload {
        let data = Data("""
        {"id":"m1","sequence":1,"kind":"file","sender_name":"Phone","created_at":"2026-08-30T00:00:00Z",
         "file":{"name":"shot.png","media_type":"image/png","size":68,"download_url":"/api/v1/files/f1"}}
        """.utf8)
        return try JSONDecoder().decode(MessagePayload.self, from: data)
    }

    /// Smallest thing UIImage will actually decode; a handful of arbitrary bytes
    /// would make the success test pass for the wrong reason.
    private static let onePixelPNG = Data(base64Encoded: """
    iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==
    """)!

    private func configuration() -> URLSessionConfiguration {
        let value = URLSessionConfiguration.ephemeral
        value.protocolClasses = [URLStub.self]
        return value
    }

    private func makeModel(client: any FerryServicing, credentials: MemoryCredentials) -> AppModel {
        let suite = "FerryTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defaults.removePersistentDomain(forName: suite)
        let model = AppModel(client: client, credentials: credentials, defaults: defaults)
        model.serverAddress = "http://127.0.0.1:8080"
        model.deviceName = "Test iPhone"
        return model
    }

    private func decodeMessage(kind: String, text: String, file: String) throws -> MessagePayload {
        let data = Data("""
        {"id":"m1","sequence":1,"kind":"\(kind)","sender_name":"Phone","created_at":"2026-08-30T00:00:00Z","text":"\(text)","file":\(file)}
        """.utf8)
        return try JSONDecoder().decode(MessagePayload.self, from: data)
    }

    private var fileJSON: String {
        "{\"name\":\"note.txt\",\"media_type\":\"text/plain\",\"size\":4,\"download_url\":\"/api/v1/files/f1\"}"
    }

    private func waitUntil(timeout: Duration = .seconds(1), condition: @escaping @MainActor () -> Bool) async {
        let clock = ContinuousClock()
        let deadline = clock.now.advanced(by: timeout)
        while !condition(), clock.now < deadline { try? await Task.sleep(for: .milliseconds(10)) }
    }
}

// Only the doubles that exercise attachments implement this. Keeping the
// default here rather than on the production protocol means a real conformance
// that forgets it is still a compile error.
@MainActor
extension FerryServicing {
    func attachment(endpoint: ServerEndpoint, token: String, path: String) async throws -> Data {
        throw FerryClient.ClientError.invalidResponse
    }
}

@MainActor
private class BaseService: FerryServicing {
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")

    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        AccessClaim(device: device, token: "token")
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
    // Declared here, not inherited from the protocol extension: a member that
    // only exists as an extension default cannot be overridden.
    func attachment(endpoint: ServerEndpoint, token: String, path: String) async throws -> Data {
        throw FerryClient.ClientError.invalidResponse
    }
}

@MainActor
private final class AttachmentService: BaseService {
    private(set) var attachmentCalls = 0
    private let data: Data?

    init(data: Data?) { self.data = data }

    override func attachment(endpoint: ServerEndpoint, token: String, path: String) async throws -> Data {
        attachmentCalls += 1
        guard let data else { throw FerryClient.ClientError.invalidResponse }
        return data
    }
}

/// Holds the response open so the test can disconnect while it is in flight.
@MainActor
private final class SuspendingAttachmentService: BaseService {
    private(set) var isLoading = false
    private let data: Data
    private var waiter: CheckedContinuation<Void, Never>?

    init(data: Data) { self.data = data }

    func release() {
        waiter?.resume()
        waiter = nil
    }

    override func attachment(endpoint: ServerEndpoint, token: String, path: String) async throws -> Data {
        isLoading = true
        await withCheckedContinuation { waiter = $0 }
        return data
    }
}

private final class URLStub: URLProtocol, @unchecked Sendable {
    nonisolated(unsafe) static var handler: ((URLRequest) throws -> (HTTPURLResponse, Data))?
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        do {
            guard let handler = Self.handler else { throw URLError(.badServerResponse) }
            let (response, data) = try handler(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch { client?.urlProtocol(self, didFailWithError: error) }
    }
    override func stopLoading() {}
}

@MainActor
private final class ScriptedService: FerryServicing {
    var messageCalls = 0
    private(set) var joinCount = 0
    private(set) var lastPassword: String?
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")

    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        joinCount += 1
        lastPassword = password
        return AccessClaim(device: device, token: "token-\(joinCount)")
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        messageCalls += 1
        if messageCalls == 2 { throw FerryClient.ClientError.unauthorized }
        return MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
}

@MainActor
private final class DelayedJoinService: FerryServicing {
    var isWaiting = false
    private var continuation: CheckedContinuation<AccessClaim, Never>?
    private let device = Device(id: "old", name: "Old iPhone", createdAt: "2026-08-30T00:00:00Z")

    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        await withCheckedContinuation { continuation in
            self.continuation = continuation
            isWaiting = true
        }
    }
    func finishJoin() {
        isWaiting = false
        continuation?.resume(returning: AccessClaim(device: device, token: "old-token"))
        continuation = nil
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
}

@MainActor
private final class DelayedAuthenticationService: FerryServicing {
    var isWaiting = false
    private var continuation: CheckedContinuation<Device, Never>?
    private let oldDevice = Device(id: "old", name: "Old iPhone", createdAt: "2026-08-30T00:00:00Z")
    private let newDevice = Device(id: "new", name: "New iPhone", createdAt: "2026-08-30T00:00:00Z")

    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        AccessClaim(device: newDevice, token: "new-token")
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device {
        await withCheckedContinuation { continuation in
            self.continuation = continuation
            isWaiting = true
        }
    }
    func finishOldAuthentication() {
        continuation?.resume(returning: oldDevice)
        continuation = nil
    }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
}

@MainActor
private final class CancellablePollingService: FerryServicing {
    var isPolling = false
    private var calls = 0
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")
    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim { AccessClaim(device: device, token: "token") }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        calls += 1
        if calls == 1 { return MessagesPage(messages: [], nextCursor: after) }
        isPolling = true
        defer { isPolling = false }
        try await Task.sleep(for: .seconds(30))
        return MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
}

@MainActor
private final class FileSendingService: FerryServicing {
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")
    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim { AccessClaim(device: device, token: "token") }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage { MessagesPage(messages: [], nextCursor: after) }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { fatalError() }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload {
        let data = Data("""
        {"id":"file","sequence":1,"kind":"file","sender_name":"Test iPhone","created_at":"2026-08-30T00:00:00Z","text":null,"file":{"name":"\(file.name)","media_type":"text/plain","size":\(file.size),"download_url":"/api/v1/files/file"}}
        """.utf8)
        return try JSONDecoder().decode(MessagePayload.self, from: data)
    }
}

@MainActor
private final class FailedSendService: FerryServicing {
    var messageCalls = 0
    let error: Error
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")
    init(error: Error) { self.error = error }
    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        AccessClaim(device: device, token: "token")
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        messageCalls += 1
        return MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload { throw error }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { throw error }
}

@MainActor
private final class DelayedSendService: FerryServicing {
    var isSendingText = false
    var isSendingFile = false
    private var textContinuation: CheckedContinuation<MessagePayload, Never>?
    private var fileContinuation: CheckedContinuation<MessagePayload, Never>?
    private var pendingText = ""
    private var pendingFile: SelectedFile?
    private let device = Device(id: "ios", name: "Test iPhone", createdAt: "2026-08-30T00:00:00Z")
    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim {
        AccessClaim(device: device, token: "token")
    }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        MessagesPage(messages: [], nextCursor: after)
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload {
        pendingText = text
        isSendingText = true
        return await withCheckedContinuation { textContinuation = $0 }
    }
    func completeText() {
        isSendingText = false
        textContinuation?.resume(returning: message(kind: "text", text: "\"\(pendingText)\"", file: "null"))
        textContinuation = nil
    }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload {
        pendingFile = file
        isSendingFile = true
        return await withCheckedContinuation { fileContinuation = $0 }
    }
    func completeFile() {
        isSendingFile = false
        let file = pendingFile!
        let payload = "{\"name\":\"\(file.name)\",\"media_type\":\"text/plain\",\"size\":\(file.size),\"download_url\":\"/api/v1/files/file\"}"
        fileContinuation?.resume(returning: message(kind: "file", text: "null", file: payload))
        fileContinuation = nil
    }
    private func message(kind: String, text: String, file: String) -> MessagePayload {
        let data = Data("""
        {"id":"\(UUID().uuidString)","sequence":1,"kind":"\(kind)","sender_name":"Test iPhone","created_at":"2026-08-30T00:00:00Z","text":\(text),"file":\(file)}
        """.utf8)
        return try! JSONDecoder().decode(MessagePayload.self, from: data)
    }
}

@MainActor
private final class RevokingDuringSendService: FerryServicing {
    var isPolling = false
    var isSending = false
    var sendWasCancelled = false
    private var messageCalls = 0
    private var pollContinuation: CheckedContinuation<MessagesPage, Error>?
    private let device = Device(id: "ios", name: "Old iPhone", createdAt: "2026-08-30T00:00:00Z")

    func join(endpoint: ServerEndpoint, password: String, name: String) async throws -> AccessClaim { AccessClaim(device: device, token: "old-token") }
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device { device }
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        messageCalls += 1
        if messageCalls == 1 { return MessagesPage(messages: [], nextCursor: after) }
        return try await withCheckedThrowingContinuation { continuation in
            pollContinuation = continuation
            isPolling = true
        }
    }
    func revoke() {
        isPolling = false
        pollContinuation?.resume(throwing: FerryClient.ClientError.unauthorized)
        pollContinuation = nil
    }
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload {
        isSending = true
        do {
            try await Task.sleep(for: .seconds(30))
            fatalError("The test send should be cancelled")
        } catch {
            isSending = false
            if error is CancellationError { sendWasCancelled = true }
            throw error
        }
    }
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload { fatalError() }
}

@MainActor
private final class MemoryCredentials: CredentialStoring {
    var token: String?
    var saveError: Error?
    func token(for origin: String) throws -> String? { token }
    func save(token: String, for origin: String) throws {
        if let saveError { throw saveError }
        self.token = token
    }
    func removeToken(for origin: String) throws { token = nil }
}

private enum TestError: LocalizedError {
    case keychain
    var errorDescription: String? { "Test Keychain failure" }
}
