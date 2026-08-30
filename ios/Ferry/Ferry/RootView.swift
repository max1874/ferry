import SwiftUI

struct RootView: View {
    @Bindable var model: AppModel

    var body: some View {
        ZStack {
            LinearGradient(colors: [Color(.systemBackground), Color.accentColor.opacity(0.08)],
                           startPoint: .top, endPoint: .bottomTrailing).ignoresSafeArea()
            if model.phase == .pairing || model.phase == .connecting { PairingView(model: model) }
            else { TimelineView(model: model) }
        }
        .task { await model.start() }
    }
}

private struct PairingView: View {
    @Bindable var model: AppModel
    @FocusState private var focused: Field?
    private enum Field { case server, name, code }

    var body: some View {
        ScrollView {
            VStack(spacing: 28) {
                Spacer(minLength: 48)
                Image(systemName: "arrow.left.arrow.right.circle.fill")
                    .font(.system(size: 66, weight: .semibold)).foregroundStyle(.cyan.gradient)
                    .accessibilityHidden(true)
                VStack(spacing: 8) {
                    Text("Ferry").font(.largeTitle.bold())
                    Text("Your clipboard and files, across your own devices.")
                        .multilineTextAlignment(.center).foregroundStyle(.secondary)
                }
                GlassEffectContainer(spacing: 16) {
                    VStack(spacing: 16) {
                        TextField("Server address", text: $model.serverAddress)
                            .textContentType(.URL).keyboardType(.URL).textInputAutocapitalization(.never)
                            .autocorrectionDisabled().focused($focused, equals: .server)
                            .accessibilityIdentifier("server-address")
                        TextField("Device name", text: $model.deviceName)
                            .focused($focused, equals: .name).accessibilityIdentifier("device-name")
                        TextField("Pairing code", text: pairingCode)
                            .textContentType(.oneTimeCode).keyboardType(.numberPad)
                            .focused($focused, equals: .code).accessibilityIdentifier("pairing-code")
                        if let message = model.statusMessage {
                            Text(message).font(.footnote).foregroundStyle(.red).frame(maxWidth: .infinity, alignment: .leading)
                                .accessibilityIdentifier("pairing-error")
                        }
                        Button { focused = nil; Task { await model.pair() } } label: {
                            HStack { if model.phase == .connecting { ProgressView() }; Text("Pair device").frame(maxWidth: .infinity) }
                        }
                        .buttonStyle(.glassProminent).controlSize(.large).disabled(model.phase == .connecting)
                        .accessibilityIdentifier("pair-device")
                    }
                    .padding(22).glassEffect(.regular, in: .rect(cornerRadius: 28))
                }
            }
            .padding(.horizontal, 24)
        }
    }

    private var pairingCode: Binding<String> {
        Binding(
            get: { model.pairingCode },
            set: { value in
                model.pairingCode = String(value.filter { $0 >= "0" && $0 <= "9" }.prefix(4))
            }
        )
    }
}
