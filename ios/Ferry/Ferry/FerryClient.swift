import Foundation

@MainActor
protocol FerryServicing {
    func claim(endpoint: ServerEndpoint, code: String, name: String) async throws -> PairingClaim
    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device
    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage
    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload
    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload
}

struct FerryClient: FerryServicing {
    enum ClientError: LocalizedError, Equatable {
        case unauthorized
        case rejected(String)
        case invalidResponse

        var errorDescription: String? {
            switch self {
            case .unauthorized: "This device is no longer paired."
            case .rejected(let message): message
            case .invalidResponse: "The Ferry Server returned an invalid response."
            }
        }
    }

    let session: URLSession
    init(session: URLSession = .shared) { self.session = session }

    func claim(endpoint: ServerEndpoint, code: String, name: String) async throws -> PairingClaim {
        try await json(endpoint: endpoint, path: "/api/v1/pairing/claim", method: "POST", token: nil,
                       body: try JSONSerialization.data(withJSONObject: ["code": code, "device_name": name]))
    }

    func currentDevice(endpoint: ServerEndpoint, token: String) async throws -> Device {
        let payload: SessionPayload = try await json(endpoint: endpoint, path: "/api/v1/session", token: token)
        return payload.device
    }

    func messages(endpoint: ServerEndpoint, token: String, after: Int64) async throws -> MessagesPage {
        try await json(endpoint: endpoint, path: "/api/v1/messages", token: token,
                       query: [URLQueryItem(name: "after", value: String(after)), URLQueryItem(name: "limit", value: "200")])
    }

    func sendText(endpoint: ServerEndpoint, token: String, text: String) async throws -> MessagePayload {
        try await json(endpoint: endpoint, path: "/api/v1/messages/text", method: "POST", token: token,
                       body: try JSONSerialization.data(withJSONObject: ["text": text]))
    }

    func sendFile(endpoint: ServerEndpoint, token: String, file: SelectedFile) async throws -> MessagePayload {
        let readTask = Task.detached(priority: .userInitiated) {
            let accessing = file.url.startAccessingSecurityScopedResource()
            defer { if accessing { file.url.stopAccessingSecurityScopedResource() } }
            let handle = try FileHandle(forReadingFrom: file.url)
            defer { try? handle.close() }
            var data = Data()
            while true {
                try Task.checkCancellation()
                guard let chunk = try handle.read(upToCount: 256 << 10), !chunk.isEmpty else { break }
                guard data.count + chunk.count <= SelectedFile.maximumBytes else {
                    throw ClientError.rejected("Files must be 64 MB or smaller.")
                }
                data.append(chunk)
            }
            return data
        }
        let data = try await withTaskCancellationHandler {
            try await readTask.value
        } onCancel: {
            readTask.cancel()
        }
        try Task.checkCancellation()
        let boundary = "Ferry-\(UUID().uuidString)"
        var body = Data("--\(boundary)\r\nContent-Disposition: form-data; name=\"file\"; filename=\"\(safeFilename(file.name))\"\r\nContent-Type: application/octet-stream\r\n\r\n".utf8)
        body.append(data)
        body.append(Data("\r\n--\(boundary)--\r\n".utf8))
        return try await json(endpoint: endpoint, path: "/api/v1/messages/file", method: "POST", token: token,
                              body: body, contentType: "multipart/form-data; boundary=\(boundary)")
    }

    private func json<T: Decodable>(endpoint: ServerEndpoint, path: String, method: String = "GET", token: String?,
                                    query: [URLQueryItem] = [], body: Data? = nil,
                                    contentType: String = "application/json") async throws -> T {
        var request = URLRequest(url: endpoint.url(path: path, queryItems: query))
        request.httpMethod = method
        request.httpBody = body
        request.cachePolicy = .reloadIgnoringLocalCacheData
        if body != nil { request.setValue(contentType, forHTTPHeaderField: "Content-Type") }
        if let token { request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        let (data, response) = try await session.data(for: request)
        guard let response = response as? HTTPURLResponse else { throw ClientError.invalidResponse }
        guard (200..<300).contains(response.statusCode) else {
            if response.statusCode == 401, token != nil { throw ClientError.unauthorized }
            if let envelope = try? JSONDecoder().decode(APIErrorEnvelope.self, from: data) { throw ClientError.rejected(envelope.error.message) }
            throw ClientError.invalidResponse
        }
        do { return try JSONDecoder().decode(T.self, from: data) }
        catch { throw ClientError.invalidResponse }
    }

    private func safeFilename(_ value: String) -> String {
        value.replacingOccurrences(of: "\\", with: "_").replacingOccurrences(of: "\"", with: "_")
            .replacingOccurrences(of: "\r", with: "_").replacingOccurrences(of: "\n", with: "_")
    }
}
