import SwiftUI

struct MessageRow: View {
    let message: MessagePayload

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Text(String(message.senderName.prefix(1)).uppercased())
                .font(.caption.bold()).frame(width: 30, height: 30)
                .background(Color.accentColor.opacity(0.18), in: .circle)
            VStack(alignment: .leading, spacing: 8) {
                Text(message.senderName).font(.subheadline.bold())
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
                    .padding(14).glassEffect(.clear, in: .rect(cornerRadius: 18))
                }
            }.frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityElement(children: .contain)
    }
}
