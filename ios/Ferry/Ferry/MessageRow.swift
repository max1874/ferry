import SwiftUI
import UIKit

struct MessageRow: View {
    let message: MessagePayload
    @State private var copied = false

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
                } else if let file = message.file {
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
            }.frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .contain)
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
