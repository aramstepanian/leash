import AppKit
import SwiftUI

enum LeashPaint {
    static let paper = Color(
        light: NSColor(srgbRed: 0.957, green: 0.945, blue: 0.925, alpha: 1),
        dark: NSColor(srgbRed: 0.110, green: 0.106, blue: 0.098, alpha: 1)
    )
    static let ink = Color(
        light: NSColor(srgbRed: 0.102, green: 0.098, blue: 0.086, alpha: 1),
        dark: NSColor(srgbRed: 0.949, green: 0.937, blue: 0.910, alpha: 1)
    )
    static let muted = ink.opacity(0.52)
    static let faint = ink.opacity(0.08)
    static let hairline = ink.opacity(0.10)
    static let well = Color(
        light: NSColor(srgbRed: 0.922, green: 0.906, blue: 0.878, alpha: 1),
        dark: NSColor(srgbRed: 0.078, green: 0.075, blue: 0.070, alpha: 1)
    )
    static let vermillion = Color(nsColor: NSColor(srgbRed: 0.839, green: 0.271, blue: 0.196, alpha: 1))
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

    var body: some View {
        HStack(spacing: 2.5) {
            ZStack {
                Circle().strokeBorder(tint, lineWidth: 1.5)
                if filled {
                    Circle()
                        .fill(tint)
                        .padding(2.4)
                }
            }
            .frame(width: 9, height: 9)
            Capsule()
                .fill(tint)
                .frame(width: 7, height: 1.5)
                .offset(y: 0.2)
        }
        .frame(width: 16, height: 16)
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
    var inverted: Bool = false

    var body: some View {
        Text(keys)
            .font(.system(size: 10, weight: .medium, design: .monospaced))
            .foregroundStyle(inverted ? LeashPaint.paper.opacity(0.72) : LeashPaint.muted)
            .padding(.horizontal, 5)
            .padding(.vertical, 2)
            .background(
                inverted ? LeashPaint.paper.opacity(0.14) : LeashPaint.faint,
                in: RoundedRectangle(cornerRadius: 4, style: .continuous)
            )
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
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.isMovableByWindowBackground = true
        window.standardWindowButton(.closeButton)?.isHidden = true
        window.standardWindowButton(.miniaturizeButton)?.isHidden = true
        window.standardWindowButton(.zoomButton)?.isHidden = true
        window.hasShadow = true
        window.level = .floating
        window.collectionBehavior.insert(.fullScreenAuxiliary)
        window.backgroundColor = .clear
        window.isOpaque = false
    }

    static func menu(_ window: NSWindow) {
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.standardWindowButton(.closeButton)?.isHidden = true
        window.standardWindowButton(.miniaturizeButton)?.isHidden = true
        window.standardWindowButton(.zoomButton)?.isHidden = true
        window.backgroundColor = .clear
        window.isOpaque = false
        window.hasShadow = true
    }
}

struct WindowAccess: NSViewRepresentable {
    var configure: (NSWindow) -> Void

    func makeNSView(context: Context) -> NSView {
        let view = NSView(frame: .zero)
        view.isHidden = true
        return view
    }

    func updateNSView(_ nsView: NSView, context: Context) {
        DispatchQueue.main.async {
            guard let window = nsView.window else { return }
            configure(window)
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
