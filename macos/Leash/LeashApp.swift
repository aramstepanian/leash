import AppKit
import SwiftUI

@main
struct LeashApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @StateObject private var app = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarView()
                .environmentObject(app)
                .onAppear {
                    delegate.model = app
                    app.start()
                }
        } label: {
            MenuBarIcon(status: app.state.status, hasPending: app.state.pending != nil)
        }
        .menuBarExtraStyle(.menu)

        Window("Leash", id: "approval") {
            ApprovalView()
                .environmentObject(app)
                .frame(width: 440)
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
        .defaultPosition(.center)
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    var model: AppModel?

    func applicationWillTerminate(_ notification: Notification) {
        model?.stop()
    }
}

struct MenuBarIcon: View {
    var status: String
    var hasPending: Bool

    var body: some View {
        Image(systemName: icon)
            .symbolRenderingMode(.hierarchical)
            .foregroundStyle(hasPending ? Color.red : Color.primary)
    }

    private var icon: String {
        if hasPending { return "hand.raised.fill" }
        if status == "watching" { return "link" }
        return "link"
    }
}
