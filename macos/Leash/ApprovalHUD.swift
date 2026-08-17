import AppKit
import SwiftUI

@MainActor
final class ApprovalHUD {
    static let shared = ApprovalHUD()

    private var panel: NSPanel?
    private var hosting: NSHostingView<AnyView>?

    func show(model: AppModel) {
        let root = AnyView(ApprovalView().environmentObject(model))
        if let hosting, let panel {
            hosting.rootView = root
            sizeToFit(panel, hosting)
            present(panel)
            return
        }

        let hosting = NSHostingView(rootView: root)
        hosting.sizingOptions = [.intrinsicContentSize]
        let panel = NSPanel(
            contentRect: NSRect(x: 0, y: 0, width: 432, height: 240),
            styleMask: [.titled, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        panel.identifier = NSUserInterfaceItemIdentifier("approval")
        panel.title = "Leash"
        panel.contentView = hosting
        panel.isReleasedWhenClosed = false
        panel.hidesOnDeactivate = false
        panel.isFloatingPanel = true
        panel.becomesKeyOnlyIfNeeded = false
        panel.animationBehavior = .utilityWindow
        self.hosting = hosting
        self.panel = panel
        sizeToFit(panel, hosting)
        present(panel)
    }

    func hide() {
        panel?.orderOut(nil)
    }

    private func sizeToFit(_ panel: NSPanel, _ hosting: NSHostingView<AnyView>) {
        hosting.invalidateIntrinsicContentSize()
        var size = hosting.fittingSize
        if !size.width.isFinite || size.width < 432 { size.width = 432 }
        if !size.height.isFinite || size.height < 168 { size.height = 220 }
        panel.setContentSize(size)
    }

    private func present(_ panel: NSPanel) {
        LeashChrome.approval(panel)
        if let screen = NSScreen.main ?? NSScreen.screens.first {
            let vis = screen.visibleFrame
            let size = panel.frame.size
            panel.setFrameOrigin(NSPoint(
                x: vis.midX - size.width / 2,
                y: vis.midY - size.height / 2 + 40
            ))
        }
        NSApp.activate(ignoringOtherApps: true)
        panel.makeKeyAndOrderFront(nil)
        if let hosting {
            panel.makeFirstResponder(hosting)
        }
        dumpShotIfRequested()
    }

    private func dumpShotIfRequested() {
        guard let dir = ProcessInfo.processInfo.environment["LEASH_SHOT"], !dir.isEmpty else { return }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.35) {
            let base = URL(fileURLWithPath: dir)
            if let frame = self.panel?.contentView?.superview {
                LeashShot.window(frame, to: base.appendingPathComponent("approval-chrome.png"))
            }
            if let view = self.panel?.contentView {
                LeashShot.window(view, to: base.appendingPathComponent("approval.png"))
            }
        }
    }
}
