import AppKit
import SwiftUI

@main
struct LeashApp: App {
    @StateObject private var app = AppModel()

    var body: some Scene {
        MenuBarExtra {
            MenuBarView()
                .environmentObject(app)
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
