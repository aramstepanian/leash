import AppKit
import SwiftUI

@main
struct LeashApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate
    @ObservedObject private var app = AppModel.shared

    var body: some Scene {
        MenuBarExtra {
            MenuBarView()
                .environmentObject(app)
        } label: {
            LeashMenuBarLabel(pending: app.state.waitingCount > 0 || app.state.mission?.phase == "act" || app.state.mission?.phase == "failed")
        }
        .menuBarExtraStyle(.window)
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.accessory)
        AppModel.shared.start()
        if let dir = ProcessInfo.processInfo.environment["LEASH_SHOT"], !dir.isEmpty {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) {
                let url = URL(fileURLWithPath: dir).appendingPathComponent("menubar.png")
                LeashShot.swiftUI(
                    MenuBarView()
                        .environmentObject(AppModel.shared)
                        .environment(\.colorScheme, .dark)
                        .frame(width: 300),
                    to: url
                )
            }
        }
    }

    func applicationWillTerminate(_ notification: Notification) {
        AppModel.shared.stop()
    }
}

struct LeashMenuBarLabel: View {
    var pending: Bool
    @State private var pulse = false

    var body: some View {
        Image(pending ? "LeashMenuFilled" : "LeashMenu")
            .renderingMode(.template)
            .interpolation(.high)
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
