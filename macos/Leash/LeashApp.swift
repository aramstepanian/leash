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
            LeashMenuBarLabel(pending: LeashFormat.menuArmed(waiting: app.state.waitingCount, phase: app.state.mission?.phase))
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
    var pending: Bool
    @State private var pulse = false

    var body: some View {
        Image(pending ? "LeashMenuFilled" : "LeashMenu")
            .renderingMode(.template)
            .interpolation(.high)
            .opacity(pending && pulse ? LeashPaint.Opacity.pulse : 1)
            .onAppear {
                pulse = pending
            }
            .onChange(of: pending) { _, isPending in
                pulse = isPending
            }
            .animation(pending ? LeashMotion.pulse : LeashMotion.settle, value: pulse)
            .accessibilityLabel(pending ? LeashCopy.waitingOnYouA11y : LeashCopy.app)
    }
}
