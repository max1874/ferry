import SwiftUI

@main
struct FerryApp: App {
    @State private var model = AppModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView(model: model)
                .onChange(of: scenePhase) { _, phase in model.setActive(phase == .active) }
        }
    }
}
