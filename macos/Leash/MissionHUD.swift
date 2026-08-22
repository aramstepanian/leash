import AppKit
import SwiftUI

@MainActor
final class MissionHUD {
    static let shared = MissionHUD()

    private var panel: NSPanel?
    private var hosting: NSHostingView<AnyView>?

    func show(model: AppModel) {
        let root = AnyView(MissionView().environmentObject(model))
        if let hosting, let panel {
            hosting.rootView = root
            present(panel)
            return
        }

        let hosting = NSHostingView(rootView: root)
        let panel = NSPanel(
            contentRect: NSRect(x: 0, y: 0, width: 720, height: 520),
            styleMask: [.titled, .fullSizeContentView, .closable],
            backing: .buffered,
            defer: false
        )
        panel.identifier = NSUserInterfaceItemIdentifier("mission")
        panel.title = "Mission Control"
        panel.contentView = hosting
        panel.isReleasedWhenClosed = false
        panel.hidesOnDeactivate = false
        panel.isFloatingPanel = true
        panel.becomesKeyOnlyIfNeeded = false
        panel.animationBehavior = .utilityWindow
        self.hosting = hosting
        self.panel = panel
        present(panel)
    }

    func hide() {
        panel?.orderOut(nil)
    }

    private func present(_ panel: NSPanel) {
        LeashChrome.mission(panel)
        if !panel.isVisible, let screen = NSScreen.main ?? NSScreen.screens.first {
            let vis = screen.visibleFrame
            let size = panel.frame.size
            panel.setFrameOrigin(NSPoint(
                x: vis.maxX - size.width - 24,
                y: vis.midY - size.height / 2
            ))
        }
        NSApp.activate(ignoringOtherApps: true)
        panel.makeKeyAndOrderFront(nil)
    }
}
