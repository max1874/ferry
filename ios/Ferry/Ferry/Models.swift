import Foundation

struct Device: Codable, Equatable, Sendable {
    let id: String
    let name: String
    let createdAt: String

    enum CodingKeys: String, CodingKey { case id, name, createdAt = "created_at" }
}

struct FileInfo: Codable, Equatable, Sendable {
    let name: String
    let mediaType: String
    let size: Int64
    let downloadURL: String

    enum CodingKeys: String, CodingKey {
        case name, size
        case mediaType = "media_type"
        case downloadURL = "download_url"
    }
}

struct MessagePayload: Codable, Equatable, Identifiable, Sendable {
    enum Kind: String, Codable, Sendable { case text, file }

    let id: String
    let sequence: Int64
    let kind: Kind
    let senderName: String
    let createdAt: String
    let text: String?
    let file: FileInfo?
    /// Whether this device sent the message. Clipboard sync uses it to leave
    /// its own sends alone; comparing sender names would confuse two devices
    /// that happen to share one.
    let isCurrentDevice: Bool

    enum CodingKeys: String, CodingKey {
        case id, sequence, kind, text, file
        case senderName = "sender_name"
        case createdAt = "created_at"
        case isCurrentDevice = "is_current_device"
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        sequence = try values.decode(Int64.self, forKey: .sequence)
        kind = try values.decode(Kind.self, forKey: .kind)
        senderName = try values.decode(String.self, forKey: .senderName)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        text = try values.decodeIfPresent(String.self, forKey: .text)
        file = try values.decodeIfPresent(FileInfo.self, forKey: .file)
        isCurrentDevice = try values.decodeIfPresent(Bool.self, forKey: .isCurrentDevice) ?? false
        guard kind == .text ? text != nil && file == nil : file != nil && text == nil else {
            throw DecodingError.dataCorrupted(.init(codingPath: values.codingPath, debugDescription: "Message payload does not match kind"))
        }
    }
}

struct AccessClaim: Decodable, Sendable { let device: Device; let token: String }
struct SessionPayload: Decodable, Sendable { let device: Device }
struct MessagesPage: Decodable, Sendable {
    let messages: [MessagePayload]
    let nextCursor: Int64
    enum CodingKeys: String, CodingKey { case messages; case nextCursor = "next_cursor" }
}

struct APIErrorEnvelope: Decodable, Sendable {
    struct Detail: Decodable, Sendable { let code: String; let message: String }
    let error: Detail
}

struct SelectedFile: Equatable, Sendable {
    nonisolated static let maximumBytes: Int64 = 64 << 20
    let url: URL
    let name: String
    let size: Int64
}
