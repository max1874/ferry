import SwiftUI
import UIKit
import UniformTypeIdentifiers

struct MessageRow: View {
    let message: MessagePayload
    let image: UIImage??
    let loadImage: () async -> Void
    let loadFile: () async -> Data?
    @State private var copied = false
    @State private var viewing = false

    private var isImage: Bool { message.file?.mediaType.hasPrefix("image/") == true }

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(String(message.senderName.prefix(1)).uppercased())
                .font(.caption.bold()).frame(width: 30, height: 30)
                .background(Color.accentColor.opacity(0.18), in: .circle)
            VStack(alignment: .leading, spacing: 8) {
                HStack(spacing: 8) {
                    Text(message.senderName).font(.subheadline.bold())
                    if let text = message.text {
                        Button(copied ? "Copied" : "Copy", systemImage: copied ? "checkmark" : "doc.on.doc") {
                            copy(text)
                        }
                        .labelStyle(.titleOnly).font(.caption).buttonStyle(.plain)
                        .foregroundStyle(.secondary).accessibilityIdentifier("copy-message")
                    }
                }
                if let text = message.text {
                    Text(text).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityIdentifier("message-text")
                } else if isImage, let file = message.file {
                    // No entry yet means the fetch is still running; an entry
                    // holding nil means it failed, and the file card stays.
                    switch image {
                    case .some(.some(let decoded)):
                        Image(uiImage: decoded).resizable().scaledToFit()
                            .frame(maxWidth: 260, maxHeight: 320)
                            .clipShape(.rect(cornerRadius: 18))
                            .onTapGesture { viewing = true }
                            .accessibilityIdentifier("message-image")
                            .accessibilityLabel(file.name)
                    case .some(.none):
                        FileCard(file: file, load: loadFile)
                    case .none:
                        RoundedRectangle(cornerRadius: 18).fill(.quaternary)
                            .frame(width: 176, height: 118)
                            .overlay { ProgressView() }
                            .task { await loadImage() }
                    }
                } else if let file = message.file {
                    FileCard(file: file, load: loadFile)
                }
            }.frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .contain)
        .fullScreenCover(isPresented: $viewing) {
            if case .some(.some(let decoded)) = image {
                ImageViewer(image: decoded, name: message.file?.name ?? "image", load: loadFile)
            }
        }
    }

    /// The only place Ferry writes the pasteboard. Nothing here runs without a
    /// tap, so there is no foreground check, no permission prompt and no state
    /// to keep in sync with the timeline.
    private func copy(_ text: String) {
        UIPasteboard.general.string = text
        copied = true
        Task {
            try? await Task.sleep(for: .seconds(1.6))
            copied = false
        }
    }
}

private struct FileCard: View {
    let file: FileInfo
    let load: () async -> Data?

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "doc.fill").foregroundStyle(.cyan)
            VStack(alignment: .leading, spacing: 2) {
                Text(file.name).font(.body.weight(.medium)).lineLimit(1)
                    .accessibilityIdentifier("message-file-name")
                Text(ByteCountFormatter.string(fromByteCount: file.size, countStyle: .file))
                    .font(.caption).foregroundStyle(.secondary)
            }
            Spacer(minLength: 12)
            SaveControl(name: file.name, mediaType: file.mediaType, load: load)
        }
        .padding(14).glassEffect(.regular, in: .rect(cornerRadius: 18))
    }
}

private struct ImageViewer: View {
    let image: UIImage
    let name: String
    let load: () async -> Data?
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            Image(uiImage: image).resizable().scaledToFit().ignoresSafeArea()
        }
        .onTapGesture { dismiss() }
        .overlay(alignment: .topTrailing) {
            HStack(spacing: 12) {
                // Saves the bytes the sender uploaded, not a re-encode of the
                // decoded UIImage, so the file keeps its original format.
                SaveControl(name: name, mediaType: "", load: load)
                Button("Close", systemImage: "xmark") { dismiss() }
                    .labelStyle(.iconOnly).buttonStyle(.glass)
            }
            .padding(20)
        }
    }
}

/// Fetches the attachment on demand and hands it to the system exporter. The
/// bytes live only until the export finishes, so saving a large file does not
/// grow the session's footprint.
private struct SaveControl: View {
    let name: String
    let mediaType: String
    let load: () async -> Data?
    @State private var exporting = false
    @State private var document: ExportedFile?
    @State private var saving = false

    private var contentType: UTType {
        UTType(mimeType: mediaType) ?? UTType(filenameExtension: (name as NSString).pathExtension) ?? .data
    }

    var body: some View {
        Button(saving ? "Saving…" : "Save", systemImage: "square.and.arrow.down") {
            saving = true
            Task {
                defer { saving = false }
                guard let data = await load() else { return }
                document = ExportedFile(data: data)
                exporting = true
            }
        }
        .labelStyle(.titleOnly).font(.caption).buttonStyle(.plain)
        .disabled(saving).accessibilityIdentifier("save-file")
        .fileExporter(isPresented: $exporting, document: document,
                      contentType: contentType, defaultFilename: name) { _ in
            document = nil
        }
    }
}

private struct ExportedFile: FileDocument {
    static var readableContentTypes: [UTType] { [.data] }
    let data: Data

    init(data: Data) { self.data = data }

    init(configuration: ReadConfiguration) throws {
        data = configuration.file.regularFileContents ?? Data()
    }

    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        FileWrapper(regularFileWithContents: data)
    }
}
