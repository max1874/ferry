import SwiftUI
import UIKit
import UniformTypeIdentifiers

struct MessageRow: View {
    let message: MessagePayload
    let image: UIImage??
    let loadImage: () async -> Void
    let loadFile: () async -> Data?
    let recalling: Bool
    let recall: () async -> Void
    @State private var confirmingRecall = false
    @State private var copied = false
    @State private var viewing = false

    private var isImage: Bool { message.file?.mediaType.hasPrefix("image/") == true }

    private var mine: Bool { message.isCurrentDevice }

    /// "iPhone · 14:08". The server writes RFC 3339 with up to nanosecond
    /// fractions, which the ISO 8601 parser rejects, and minutes are enough.
    private var meta: String {
        let date = try? Date(message.createdAt.replacing(/\.\d+/, with: ""), strategy: .iso8601)
        return [message.senderName, date?.formatted(date: .omitted, time: .shortened)].compactMap(\.self).joined(separator: " · ")
    }

    // Other devices' text reads as plain full-width text; this device's sends sit
    // in a bubble on the trailing side, as in a chat. Attachments keep their shape.
    var body: some View {
        VStack(alignment: mine ? .trailing : .leading, spacing: 6) {
            Text(meta).font(.caption).foregroundStyle(.secondary).lineLimit(1)
            Group {
                if let text = message.text {
                    if mine {
                        Text(text).textSelection(.enabled)
                            .padding(.horizontal, 14).padding(.vertical, 9)
                            .background(Color(.systemGray6), in: .rect(cornerRadius: 20))
                            .accessibilityIdentifier("message-text")
                    } else {
                        Text(text).textSelection(.enabled).frame(maxWidth: .infinity, alignment: .leading)
                            .accessibilityIdentifier("message-text")
                    }
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
            }
            .padding(mine ? .leading : .trailing, mine ? 56 : 0)
            actions
        }
        .frame(maxWidth: .infinity, alignment: mine ? .trailing : .leading)
        .confirmationDialog("Recall this message from all devices?", isPresented: $confirmingRecall, titleVisibility: .visible) {
            Button("Recall", role: .destructive) { Task { await recall() } }
            Button("Cancel", role: .cancel) { }
        } message: {
            Text("Copies and downloads already saved will remain.")
        }
        .accessibilityElement(children: .contain)
        .fullScreenCover(isPresented: $viewing) {
            if case .some(.some(let decoded)) = image {
                ImageViewer(image: decoded, name: message.file?.name ?? "image", load: loadFile)
            }
        }
    }

    /// Icon-only actions under the content, compact like ChatGPT's: 36-point
    /// targets, under the 44-point guideline in exchange for a row that stays
    /// out of the way. The negative padding puts the outer glyph on the
    /// content's edge and keeps the targets' height from spacing out messages.
    private var actions: some View {
        HStack(spacing: 0) {
            if let text = message.text {
                Button { copy(text) } label: { actionLabel(copied ? "Copied" : "Copy", copied ? "checkmark" : "square.on.square") }
                    .contentTransition(.symbolEffect(.replace))
                    .accessibilityIdentifier("copy-message")
            }
            Button { confirmingRecall = true } label: { actionLabel(recalling ? "Recalling…" : "Recall", "trash") }
                .disabled(recalling)
                .accessibilityIdentifier("recall-message")
        }
        .labelStyle(.iconOnly).buttonStyle(.plain).font(.callout).foregroundStyle(.secondary)
        .padding(mine ? .trailing : .leading, -9).padding(.vertical, -6)
    }

    // The frame sits inside the label because a plain button hit-tests its label.
    private func actionLabel(_ title: String, _ systemImage: String) -> some View {
        Label(title, systemImage: systemImage).frame(width: 36, height: 36).contentShape(.rect)
    }

    /// The only place Ferry writes the pasteboard. Nothing here runs without a
    /// tap, so there is no foreground check, no permission prompt and no state
    /// to keep in sync with the timeline.
    private func copy(_ text: String) {
        UIPasteboard.general.string = text
        withAnimation { copied = true }
        Task {
            try? await Task.sleep(for: .seconds(1.6))
            withAnimation { copied = false }
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
            SaveControl(name: file.name, load: load)
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
            Image(uiImage: image).resizable().scaledToFit()
        }
        .onTapGesture { dismiss() }
        .overlay(alignment: .topTrailing) {
            HStack(spacing: 12) {
                // Saves the bytes the sender uploaded, not a re-encode of the
                // decoded UIImage, so the file keeps its original format.
                SaveControl(name: name, load: load)
                Button("Close", systemImage: "xmark") { dismiss() }
                    .labelStyle(.iconOnly).buttonStyle(.glass)
            }
            .padding(20)
        }
        .preferredColorScheme(.dark)
    }
}

/// Fetches the attachment on demand and hands it to the system exporter. The
/// bytes live only until the export finishes, so saving a large file does not
/// grow the session's footprint.
private struct SaveControl: View {
    let name: String
    let load: () async -> Data?
    @State private var exporting = false
    @State private var document: ExportedFile?
    @State private var saving = false

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
                      contentTypes: ExportedFile.writableContentTypes, defaultFilename: name) { _ in
            document = nil
        }
    }
}

private struct ExportedFile: FileDocument {
    // Export opaque original bytes. The original filename supplies the extension;
    // the exporter uses this document's supported types instead of inferring a format.
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
