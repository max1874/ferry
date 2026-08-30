import SwiftUI
import UniformTypeIdentifiers

struct TimelineView: View {
    @Bindable var model: AppModel
    @State private var importingFile = false

    var body: some View {
        NavigationStack {
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(spacing: 22) {
                        if model.messages.isEmpty {
                            ContentUnavailableView("Ready to ferry", systemImage: "shippingbox",
                                                   description: Text("Send text or a file to your other devices."))
                                .padding(.top, 100)
                        }
                        ForEach(model.messages) { MessageRow(message: $0).id($0.id) }
                    }.padding(.horizontal, 18).padding(.vertical, 20)
                }
                .onChange(of: model.messages.count) { _, _ in
                    if let id = model.messages.last?.id { withAnimation { proxy.scrollTo(id, anchor: .bottom) } }
                }
            }
            .navigationTitle("Ferry")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Label(model.currentDevice?.name ?? "Device", systemImage: "iphone")
                        .font(.subheadline.weight(.semibold)).accessibilityIdentifier("current-device")
                }
            }
            .safeAreaInset(edge: .bottom) { composer }
        }
        .fileImporter(isPresented: $importingFile, allowedContentTypes: [.data], allowsMultipleSelection: false) { result in
            switch result {
            case .success(let urls):
                if let url = urls.first { model.selectFile(url) }
            case .failure(let error):
                model.fileImportFailed(error)
            }
        }
    }

    private var composer: some View {
        VStack(spacing: 8) {
            if let warning = model.credentialWarning {
                Label(warning, systemImage: "exclamationmark.triangle")
                    .font(.footnote).foregroundStyle(.red)
            }
            if let message = model.statusMessage {
                Label(message, systemImage: model.phase == .offline ? "wifi.exclamationmark" : "exclamationmark.triangle")
                    .font(.footnote).foregroundStyle(model.phase == .offline ? .orange : .red)
            }
            if let error = model.sendError {
                Label(error, systemImage: "arrow.up.circle")
                    .font(.footnote).foregroundStyle(.red)
            }
            if model.phase == .offline {
                Button("Change Server", systemImage: "network") { model.disconnect() }
                    .buttonStyle(.glass).accessibilityIdentifier("change-server")
            }
            if let file = model.selectedFile {
                HStack {
                    Label(file.name, systemImage: "doc").lineLimit(1)
                    Spacer()
                    Text(ByteCountFormatter.string(fromByteCount: file.size, countStyle: .file)).foregroundStyle(.secondary)
                    Button("Remove", systemImage: "xmark.circle.fill") { model.clearSelectedFile() }.labelStyle(.iconOnly)
                }
                .font(.footnote).padding(.horizontal, 14).padding(.vertical, 9)
                .glassEffect(.regular, in: .capsule).accessibilityIdentifier("selected-file")
            }
            GlassEffectContainer(spacing: 10) {
                HStack(alignment: .bottom, spacing: 10) {
                    Button("Attach file", systemImage: "plus") { importingFile = true }
                        .labelStyle(.iconOnly).buttonStyle(.glass).controlSize(.large)
                        .accessibilityIdentifier("attach-file")
                    TextField("Message", text: $model.draft, axis: .vertical)
                        .lineLimit(1...5).padding(.horizontal, 4).padding(.vertical, 10)
                        .disabled(model.selectedFile != nil).accessibilityIdentifier("message-input")
                    Button("Send", systemImage: "arrow.up") { Task { await model.send() } }
                        .labelStyle(.iconOnly).buttonStyle(.glassProminent).controlSize(.large)
                        .disabled(!model.canSend).accessibilityIdentifier("send-message")
                }
                .padding(10).glassEffect(.regular.interactive(), in: .rect(cornerRadius: 28))
            }
        }
        .padding(.horizontal, 12).padding(.bottom, 6)
    }
}
