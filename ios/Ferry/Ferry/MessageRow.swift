import SwiftUI
import UIKit

struct MessageRow: View {
    let message: MessagePayload
    let image: UIImage??
    let loadImage: () async -> Void
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
                        FileCard(file: file)
                    case .none:
                        RoundedRectangle(cornerRadius: 18).fill(.quaternary)
                            .frame(width: 176, height: 118)
                            .overlay { ProgressView() }
                            .task { await loadImage() }
                    }
                } else if let file = message.file {
                    FileCard(file: file)
                }
            }.frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .contain)
        .fullScreenCover(isPresented: $viewing) {
            if case .some(.some(let decoded)) = image { ImageViewer(image: decoded) }
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

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "doc.fill").foregroundStyle(.cyan)
            VStack(alignment: .leading, spacing: 2) {
                Text(file.name).font(.body.weight(.medium)).lineLimit(1)
                    .accessibilityIdentifier("message-file-name")
                Text(ByteCountFormatter.string(fromByteCount: file.size, countStyle: .file))
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(14).glassEffect(.regular, in: .rect(cornerRadius: 18))
    }
}

private struct ImageViewer: View {
    let image: UIImage
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            Image(uiImage: image).resizable().scaledToFit().ignoresSafeArea()
        }
        .onTapGesture { dismiss() }
        .overlay(alignment: .topTrailing) {
            Button("Close", systemImage: "xmark") { dismiss() }
                .labelStyle(.iconOnly).buttonStyle(.glass).padding(20)
        }
    }
}
