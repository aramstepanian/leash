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
            LeashMenuBarLabel(running: app.state.job?.running == true)
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
            DispatchQueue.main.asyncAfter(deadline: .now() + LeashMotion.launchShot) {
                let url = URL(fileURLWithPath: dir).appendingPathComponent("menubar.png")
                LeashShot.swiftUI(
                    MenuBarView()
                        .environmentObject(AppModel.shared)
                        .environment(\.colorScheme, .dark)
                        .frame(width: LeashLayout.menuWidth),
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
    var running: Bool
    @State private var pulse = false

    var body: some View {
        Image(running ? "LeashMenuFilled" : "LeashMenu")
            .renderingMode(.template)
            .interpolation(.high)
            .opacity(running && pulse ? LeashPaint.Opacity.pulse : 1)
            .onAppear {
                pulse = running
            }
            .onChange(of: running) { _, isRunning in
                pulse = isRunning
            }
            .animation(running ? LeashMotion.pulse : LeashMotion.settle, value: pulse)
            .accessibilityLabel(running ? LeashCopy.runningA11y : LeashCopy.app)
    }
}
