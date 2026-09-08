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
                ToolbarItem(placement: .topBarLeading) {
                    Menu {
                        Toggle("Sync to clipboard", systemImage: "doc.on.clipboard", isOn: $model.clipboardSyncEnabled)
                            .accessibilityIdentifier("clipboard-sync-toggle")
                        Text("Ferry replaces this clipboard when another device sends text or an image while Ferry is open.")
                    } label: {
                        Label("Settings", systemImage: "ellipsis.circle")
                    }
                    .accessibilityIdentifier("settings-menu")
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
            if let clipboard = model.clipboardStatus {
                Label(clipboard, systemImage: "doc.on.clipboard")
                    .font(.footnote).foregroundStyle(.secondary)
                    .accessibilityIdentifier("clipboard-status")
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
                    // The system paste control is the only way an iOS app gets
                    // pasteboard content without interrogating the user with a
                    // permission alert on every read.
                    PasteButton(supportedContentTypes: [.png, .jpeg, .utf8PlainText]) { providers in
                        Task { await paste(providers) }
                    }
                    .labelStyle(.iconOnly).buttonBorderShape(.circle).controlSize(.large)
                    .disabled(model.isSending).accessibilityIdentifier("paste-clipboard")
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

    /// An image is worth more than its text alternative, so images win when the
    /// paste carries both.
    private func paste(_ providers: [NSItemProvider]) async {
        for (type, suffix) in [(UTType.png, "png"), (UTType.jpeg, "jpg")] {
            guard let provider = providers.first(where: { $0.hasItemConformingToTypeIdentifier(type.identifier) }),
                  let data = await load(type, from: provider) else { continue }
            await model.sendPasted(imageData: data, name: "clipboard-\(Int(Date.now.timeIntervalSince1970)).\(suffix)")
            return
        }
        for provider in providers {
            guard let data = await load(.utf8PlainText, from: provider),
                  let text = String(data: data, encoding: .utf8) else { continue }
            await model.sendPasted(text: text)
            return
        }
    }

    /// `NSItemProvider` reports progress instead of returning Void, so Swift
    /// does not synthesise an async form of it and the bridge is written out.
    private func load(_ type: UTType, from provider: NSItemProvider) async -> Data? {
        await withCheckedContinuation { continuation in
            _ = provider.loadDataRepresentation(for: type) { data, _ in
                continuation.resume(returning: data)
            }
        }
    }
}
