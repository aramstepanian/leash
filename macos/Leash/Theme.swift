import AppKit
import SwiftUI

enum LeashChrome {
    static func floating(_ window: NSWindow) {
        stripTitlebar(window)
        window.isMovableByWindowBackground = true
        window.hasShadow = true
        window.level = .floating
        window.collectionBehavior.insert(.fullScreenAuxiliary)
        window.backgroundColor = LeashPaint.paperNS
        window.isOpaque = true
    }

    static func approval(_ window: NSWindow) { floating(window) }
    static func mission(_ window: NSWindow) { floating(window) }

    static func menu(_ window: NSWindow) {
        stripTitlebar(window)
        window.hasShadow = true
        window.backgroundColor = LeashPaint.paperNS
        window.isOpaque = true
    }

    private static func stripTitlebar(_ window: NSWindow) {
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.styleMask.insert(.fullSizeContentView)
        window.standardWindowButton(.closeButton)?.isHidden = true
        window.standardWindowButton(.miniaturizeButton)?.isHidden = true
        window.standardWindowButton(.zoomButton)?.isHidden = true
        window.standardWindowButton(.closeButton)?.superview?.isHidden = true
    }
}

struct LeashWindowFill: ViewModifier {
    func body(content: Content) -> some View {
        if #available(macOS 15.0, *) {
            content
                .background(LeashPaint.paper.ignoresSafeArea())
                .containerBackground(LeashPaint.paper, for: .window)
        } else {
            content
                .background(LeashPaint.paper.ignoresSafeArea())
        }
    }
}

extension View {
    func leashWindowFill() -> some View {
        modifier(LeashWindowFill())
    }

    func leashChrome(_ configure: @escaping (NSWindow) -> Void) -> some View {
        leashWindowFill()
            .background(WindowAccess(configure: configure))
    }
}

struct WindowAccess: NSViewRepresentable {
    var configure: (NSWindow) -> Void

    func makeNSView(context: Context) -> WindowProbe {
        let view = WindowProbe()
        view.configure = configure
        return view
    }

    func updateNSView(_ nsView: WindowProbe, context: Context) {
        nsView.configure = configure
        nsView.apply()
    }
}

final class WindowProbe: NSView {
    var configure: (NSWindow) -> Void = { _ in }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        apply()
    }

    override func hitTest(_ point: NSPoint) -> NSView? { nil }

    func apply() {
        guard let window else { return }
        configure(window)
        DispatchQueue.main.async { [weak self] in
            guard let window = self?.window else { return }
            self?.configure(window)
        }
    }
}

func compactPath(_ path: String) -> String {
    LeashFormat.compactPath(path)
}

func highlightedCommand(_ text: String, needles: [String], accent: Color) -> AttributedString {
    var result = AttributedString(text)
    result.font = LeashType.codeMedium
    result.foregroundColor = LeashPaint.code
    for needle in needles {
        let token = needle.trimmingCharacters(in: .whitespacesAndNewlines)
        guard token.count >= LeashLayout.highlightFloor else { continue }
        var search = text.startIndex
        while let found = text.range(of: token, options: .caseInsensitive, range: search..<text.endIndex) {
            if let low = AttributedString.Index(found.lowerBound, within: result),
               let high = AttributedString.Index(found.upperBound, within: result) {
                result[low..<high].foregroundColor = accent
                result[low..<high].font = LeashType.codeStrong
            }
            search = found.upperBound
        }
    }
    return result
}

enum LeashShot {
    static func window(_ view: NSView, to url: URL) {
        let bounds = view.bounds
        guard bounds.width > 1, bounds.height > 1,
              let rep = view.bitmapImageRepForCachingDisplay(in: bounds)
        else { return }
        view.cacheDisplay(in: bounds, to: rep)
        guard let png = rep.representation(using: .png, properties: [:]) else { return }
        try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? png.write(to: url)
    }

    @MainActor
    static func swiftUI<V: View>(_ view: V, to url: URL) {
        let renderer = ImageRenderer(content: view)
        renderer.scale = LeashLayout.shotScale
        guard let ns = renderer.nsImage,
              let tiff = ns.tiffRepresentation,
              let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:])
        else { return }
        try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? png.write(to: url)
    }
}
