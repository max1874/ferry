import SwiftUI

struct RootView: View {
    @Bindable var model: AppModel

    var body: some View {
        ZStack {
            LinearGradient(colors: [Color(.systemBackground), Color.accentColor.opacity(0.08)],
                           startPoint: .top, endPoint: .bottomTrailing).ignoresSafeArea()
            if model.phase == .setup || model.phase == .connecting { AccessView(model: model) }
            else { TimelineView(model: model) }
        }
        .task { await model.start() }
    }
}

private struct AccessView: View {
    @Bindable var model: AppModel
    @FocusState private var focused: Field?
    private enum Field { case server, name, password }

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
                        SecureField("Password (if enabled)", text: $model.accessPassword)
                            .textContentType(.password)
                            .focused($focused, equals: .password).accessibilityIdentifier("access-password")
                        if let message = model.statusMessage {
                            Text(message).font(.footnote).foregroundStyle(.red).frame(maxWidth: .infinity, alignment: .leading)
                                .accessibilityIdentifier("access-error")
                        }
                        Button { focused = nil; Task { await model.connect() } } label: {
                            HStack { if model.phase == .connecting { ProgressView() }; Text("Connect").frame(maxWidth: .infinity) }
                        }
                        .buttonStyle(.glassProminent).controlSize(.large)
                        .disabled(model.phase == .connecting)
                        .accessibilityIdentifier("connect-device")
                    }
                    .padding(22).glassEffect(.regular, in: .rect(cornerRadius: 28))
                }
            }
            .padding(.horizontal, 24)
        }
    }
}
