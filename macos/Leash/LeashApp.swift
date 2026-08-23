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
            LeashMenuBarLabel(
                mode: LeashMenuMode.resolve(
                    waiting: app.state.waitingCount,
                    working: LeashFormat.missionLive(phase: app.state.mission?.phase, pending: app.state.pending != nil) && app.state.waitingCount == 0,
                    watching: DaemonStatus(app.state.status) == .watching,
                    offline: app.daemonError != nil,
                    connecting: app.connecting
                )
            )
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
