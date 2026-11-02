import Foundation
import UniformTypeIdentifiers

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
    let isCurrentDevice: Bool
    let text: String?
    let file: FileInfo?

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
        isCurrentDevice = try values.decodeIfPresent(Bool.self, forKey: .isCurrentDevice) ?? false
        text = try values.decodeIfPresent(String.self, forKey: .text)
        file = try values.decodeIfPresent(FileInfo.self, forKey: .file)
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
    let retractedIDs: [String]
    let nextRetractionCursor: Int64
    enum CodingKeys: String, CodingKey {
        case messages
        case nextCursor = "next_cursor"
        case retractedIDs = "retracted_ids"
        case nextRetractionCursor = "next_retraction_cursor"
    }

    init(messages: [MessagePayload], nextCursor: Int64, retractedIDs: [String] = [], nextRetractionCursor: Int64 = 0) {
        self.messages = messages
        self.nextCursor = nextCursor
        self.retractedIDs = retractedIDs
        self.nextRetractionCursor = nextRetractionCursor
    }

    init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        messages = try values.decode([MessagePayload].self, forKey: .messages)
        nextCursor = try values.decode(Int64.self, forKey: .nextCursor)
        retractedIDs = try values.decodeIfPresent([String].self, forKey: .retractedIDs) ?? []
        nextRetractionCursor = try values.decodeIfPresent(Int64.self, forKey: .nextRetractionCursor) ?? 0
    }
}

struct APIErrorEnvelope: Decodable, Sendable {
    struct Detail: Decodable, Sendable { let code: String; let message: String }
    let error: Detail
}

struct SelectedFile: Equatable, Sendable {
    nonisolated static let maximumBytes: Int64 = 64 << 20

    /// A picked file is read from disk when it is sent. A pasted image has no
    /// file behind it, so its bytes travel with the selection and are released
    /// with it.
    enum Contents: Equatable, Sendable {
        case file(URL)
        case pasted(Data)
    }

    let contents: Contents
    let name: String
    let size: Int64
    /// Sent as the upload part's Content-Type. The Server stores it as given,
    /// and every client previews a file only when it says `image/*`.
    let mediaType: String

    nonisolated init(url: URL, name: String, size: Int64) {
        contents = .file(url)
        self.name = name
        self.size = size
        mediaType = UTType(filenameExtension: (name as NSString).pathExtension)?.preferredMIMEType ?? "application/octet-stream"
    }

    nonisolated init(pasted data: Data, name: String, mediaType: String) {
        contents = .pasted(data)
        self.name = name
        size = Int64(data.count)
        self.mediaType = mediaType
    }

    nonisolated var url: URL? {
        if case .file(let url) = contents { url } else { nil }
    }
}
