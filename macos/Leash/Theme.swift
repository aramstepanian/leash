import AppKit
import SwiftUI

enum LeashPaint {
    static let paperNS = NSColor(name: nil, dynamicProvider: { appearance in
        appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            ? NSColor(srgbRed: 0.110, green: 0.106, blue: 0.098, alpha: 1)
            : NSColor(srgbRed: 0.957, green: 0.945, blue: 0.925, alpha: 1)
    })
    static let paper = Color(nsColor: paperNS)
    static let ink = Color(
        light: NSColor(srgbRed: 0.102, green: 0.098, blue: 0.086, alpha: 1),
        dark: NSColor(srgbRed: 0.949, green: 0.937, blue: 0.910, alpha: 1)
    )
    static let muted = ink.opacity(0.52)
    static let faint = ink.opacity(0.08)
    static let hairline = ink.opacity(0.14)
    static let well = Color(
        light: NSColor(srgbRed: 0.922, green: 0.906, blue: 0.878, alpha: 1),
        dark: NSColor(srgbRed: 0.078, green: 0.075, blue: 0.070, alpha: 1)
    )
    static let vermillion = Color(nsColor: NSColor(srgbRed: 0.839, green: 0.271, blue: 0.196, alpha: 1))
    static let bone = Color(nsColor: NSColor(srgbRed: 0.969, green: 0.953, blue: 0.933, alpha: 1))
    static let amber = Color(nsColor: NSColor(srgbRed: 0.788, green: 0.518, blue: 0.165, alpha: 1))
    static let steel = Color(nsColor: NSColor(srgbRed: 0.353, green: 0.478, blue: 0.659, alpha: 1))
}

enum LeashKind {
    case destroy, secret, outside, other

    init(_ raw: String) {
        switch raw {
        case "destroy": self = .destroy
        case "secret": self = .secret
        case "outside": self = .outside
        default: self = .other
        }
    }

    var label: String {
        switch self {
        case .destroy: return "Danger"
        case .secret: return "Secret"
        case .outside: return "Outside"
        case .other: return "Ask"
        }
    }

    var color: Color {
        switch self {
        case .destroy: return LeashPaint.vermillion
        case .secret: return LeashPaint.amber
        case .outside: return LeashPaint.steel
        case .other: return LeashPaint.ink.opacity(0.7)
        }
    }
}

struct LeashMark: View {
    var filled: Bool = false
    var tint: Color = LeashPaint.ink
    var size: CGFloat = 16

    var body: some View {
        let line = max(1.4, size * 0.12)
        let circle = size * 0.50
        let strap = size * 0.30
        let gap = line * 0.55
        HStack(spacing: gap) {
            ZStack {
                Circle().strokeBorder(tint, lineWidth: line)
                if filled {
                    Circle()
                        .fill(tint)
                        .padding(line * 0.95)
                }
            }
            .frame(width: circle, height: circle)
            Capsule()
                .fill(tint)
                .frame(width: strap, height: line)
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

struct LeashWordmark: View {
    var body: some View {
        Text("Leash")
            .font(.system(size: 10, weight: .semibold))
            .tracking(1.8)
            .textCase(.uppercase)
            .foregroundStyle(LeashPaint.muted)
    }
}

struct KeyHint: View {
    var keys: String
    var on: Tone = .paper

    enum Tone { case paper, ink, vermillion }

    var body: some View {
        Text(keys)
            .font(.system(size: 10, weight: .medium, design: .monospaced))
            .foregroundStyle(foreground)
            .padding(.horizontal, 5)
            .padding(.vertical, 2)
            .background(background, in: RoundedRectangle(cornerRadius: 4, style: .continuous))
    }

    private var foreground: Color {
        switch on {
        case .paper: return LeashPaint.muted
        case .ink: return LeashPaint.paper.opacity(0.72)
        case .vermillion: return LeashPaint.bone.opacity(0.82)
        }
    }

    private var background: Color {
        switch on {
        case .paper: return LeashPaint.faint
        case .ink: return LeashPaint.paper.opacity(0.14)
        case .vermillion: return LeashPaint.bone.opacity(0.18)
        }
    }
}

struct KindChip: View {
    var kind: LeashKind

    var body: some View {
        Text(kind.label)
            .font(.system(size: 10, weight: .semibold))
            .tracking(0.4)
            .foregroundStyle(kind.color)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(kind.color.opacity(0.12), in: Capsule())
    }
}

struct Hairline: View {
    var body: some View {
        Rectangle()
            .fill(LeashPaint.hairline)
            .frame(height: 1)
    }
}

enum LeashChrome {
    static func approval(_ window: NSWindow) {
        stripTitlebar(window)
        window.isMovableByWindowBackground = true
        window.hasShadow = true
        window.level = .floating
        window.collectionBehavior.insert(.fullScreenAuxiliary)
        window.backgroundColor = LeashPaint.paperNS
        window.isOpaque = true
    }

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
    let home = FileManager.default.homeDirectoryForCurrentUser.path
    if path.hasPrefix(home) {
        return "~" + path.dropFirst(home.count)
    }
    return path
}

func highlightedCommand(_ text: String, needles: [String], accent: Color) -> AttributedString {
    var result = AttributedString(text)
    result.font = .system(size: 12.5, weight: .medium, design: .monospaced)
    result.foregroundColor = LeashPaint.ink.opacity(0.9)
    for needle in needles {
        let token = needle.trimmingCharacters(in: .whitespacesAndNewlines)
        guard token.count >= 2 else { continue }
        var search = text.startIndex
        while let found = text.range(of: token, options: .caseInsensitive, range: search..<text.endIndex) {
            if let low = AttributedString.Index(found.lowerBound, within: result),
               let high = AttributedString.Index(found.upperBound, within: result) {
                result[low..<high].foregroundColor = accent
                result[low..<high].font = .system(size: 12.5, weight: .semibold, design: .monospaced)
            }
            search = found.upperBound
        }
    }
    return result
}

extension Color {
    init(light: NSColor, dark: NSColor) {
        self.init(nsColor: NSColor(name: nil, dynamicProvider: { appearance in
            appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? dark : light
        }))
    }
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
        renderer.scale = 2
        guard let ns = renderer.nsImage,
              let tiff = ns.tiffRepresentation,
              let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:])
        else { return }
        try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? png.write(to: url)
    }
}
