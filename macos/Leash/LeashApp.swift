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
            LeashMenuBarLabel(pending: app.state.pending != nil)
        }
        .menuBarExtraStyle(.window)

        Window("Leash", id: "approval") {
            ApprovalView()
                .environmentObject(app)
        }
        .windowStyle(.hiddenTitleBar)
        .windowResizability(.contentSize)
        .defaultPosition(.center)
        .defaultSize(width: 432, height: 320)
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    var model: AppModel?

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
    }

    func applicationWillTerminate(_ notification: Notification) {
        model?.stop()
    }
}

struct LeashMenuBarLabel: View {
    var pending: Bool
    @State private var pulse = false

    var body: some View {
        LeashMark(filled: pending, tint: .primary)
            .opacity(pending && pulse ? 0.35 : 1)
            .onAppear {
                pulse = pending
            }
            .onChange(of: pending) { _, isPending in
                pulse = isPending
            }
            .animation(
                pending ? .easeInOut(duration: 0.9).repeatForever(autoreverses: true) : .easeOut(duration: 0.15),
                value: pulse
            )
            .accessibilityLabel(pending ? "Leash — waiting on you" : "Leash")
    }
}
