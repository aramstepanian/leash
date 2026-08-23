import SwiftUI

struct LeashClipGeom {
    let ring: CGRect
    let strap: CGRect
    let line: CGFloat

    var center: CGPoint { CGPoint(x: ring.midX, y: ring.midY) }
    var radius: CGFloat { max(0, ring.width / 2 - line / 2) }

    init(_ size: CGSize) {
        let s = min(size.width, size.height)
        let d = s * LeashSpace.Mark.span
        let line = max(LeashSpace.Mark.minStroke, d * LeashSpace.Mark.stroke)
        let strapH = max(line * LeashSpace.Mark.strapLine, d * LeashSpace.Mark.strapHeight)
        let strapW = d * LeashSpace.Mark.strap
        let overlap = line * LeashSpace.Mark.overlap
        let groupW = d - overlap + strapW
        let x0 = (size.width - groupW) / 2
        let y0 = (size.height - d) / 2
        ring = CGRect(x: x0, y: y0, width: d, height: d)
        strap = CGRect(
            x: x0 + d - overlap,
            y: y0 + (d - strapH) / 2,
            width: strapW,
            height: strapH
        )
        self.line = line
    }
}

enum LeashClipMode: Equatable {
    case rest(filled: Bool)
    case spin
    case fasten
    case pulse
}

enum LeashActivity: Equatable {
    case rest, connecting, working, waiting
}

enum LeashMenuMode: Equatable {
    case connecting, idle, watching, working, waiting, offline

    static func resolve(waiting: Int, working: Bool, watching: Bool, offline: Bool, connecting: Bool) -> LeashMenuMode {
        if connecting && !offline { return .connecting }
        if offline { return .offline }
        if waiting > 0 { return .waiting }
        if working { return .working }
        if watching { return .watching }
        return .idle
    }

    var clip: LeashClipMode {
        switch self {
        case .connecting: return .fasten
        case .working: return .spin
        case .waiting: return .pulse
        case .watching: return .rest(filled: true)
        case .idle, .offline: return .rest(filled: false)
        }
    }

    var dimmed: Bool { self == .offline }

    var accessibility: String {
        switch self {
        case .connecting: return LeashCopy.starting
        case .waiting: return LeashCopy.waitingOnYouA11y
        case .working: return LeashCopy.working
        case .watching: return LeashCopy.watching
        case .offline: return LeashCopy.offline
        case .idle: return LeashCopy.app
        }
    }
}

enum LeashLoaderSize {
    case tick, menu, badge, inline, panel, hero

    var points: CGFloat {
        switch self {
        case .tick: return 14
        case .menu: return 18
        case .badge: return 22
        case .inline: return 28
        case .panel: return 40
        case .hero: return 56
        }
    }
}

struct LeashMark: View {
    var filled: Bool = false
    var tint: Color = LeashPaint.ink
    var size: CGFloat = LeashSpace.mark

    var body: some View {
        LeashClip(mode: .rest(filled: filled), tint: tint)
            .frame(width: size, height: size)
            .accessibilityHidden(true)
    }
}

struct LeashLoader: View {
    var mode: LeashClipMode = .spin
    var size: LeashLoaderSize = .inline
    var tint: Color = LeashPaint.ink

    var body: some View {
        LeashClip(mode: mode, tint: tint)
            .frame(width: size.points, height: size.points)
            .accessibilityHidden(true)
    }
}

struct LeashMenuBarLabel: View {
    var mode: LeashMenuMode

    var body: some View {
        LeashClip(mode: mode.clip, tint: .primary)
            .frame(width: LeashLoaderSize.menu.points, height: LeashLoaderSize.menu.points)
            .opacity(mode.dimmed ? LeashPaint.Opacity.disabled : 1)
            .accessibilityLabel(mode.accessibility)
    }
}

struct LeashClip: View {
    var mode: LeashClipMode
    var tint: Color = LeashPaint.ink

    var body: some View {
        Group {
            if animates {
                TimelineView(.animation(minimumInterval: 1.0 / 30.0)) { timeline in
                    canvas(at: timeline.date)
                }
            } else {
                canvas(at: .now)
            }
        }
    }

    private var animates: Bool {
        switch mode {
        case .spin, .fasten, .pulse: return true
        case .rest: return false
        }
    }

    private func canvas(at date: Date) -> some View {
        Canvas { context, size in
            let g = LeashClipGeom(size)
            let t = date.timeIntervalSinceReferenceDate
            switch mode {
            case .rest(let filled):
                drawMark(context: &context, geom: g, tint: tint, filled: filled, seated: 1)
            case .spin:
                let turns = t / LeashMotion.spinPeriod
                let start = Angle.degrees(turns.truncatingRemainder(dividingBy: 1) * 360 - 90)
                drawArc(context: &context, geom: g, tint: tint, start: start, sweep: .degrees(LeashSpace.Mark.spinSweep))
                drawStrap(context: &context, geom: g, tint: tint, seated: 1)
            case .fasten:
                let u = t.truncatingRemainder(dividingBy: LeashMotion.fastenPeriod) / LeashMotion.fastenPeriod
                let seated = fastenSeated(u)
                drawArc(
                    context: &context,
                    geom: g,
                    tint: tint,
                    start: .degrees(-90),
                    sweep: .degrees(220 + 140 * seated)
                )
                drawStrap(context: &context, geom: g, tint: tint, seated: seated)
            case .pulse:
                let wave = 0.5 + 0.5 * sin(t * .pi * 2 / LeashMotion.pulsePeriod)
                var pulsing = context
                pulsing.opacity = 0.45 + 0.55 * wave
                drawMark(context: &pulsing, geom: g, tint: tint, filled: true, seated: 1)
            }
        }
    }

    private func fastenSeated(_ u: Double) -> CGFloat {
        let p: Double
        if u < 0.38 {
            p = smooth(u / 0.38)
        } else if u < 0.54 {
            p = 1
        } else if u < 0.90 {
            p = 1 - smooth((u - 0.54) / 0.36)
        } else {
            p = 0
        }
        return 0.18 + 0.82 * CGFloat(p)
    }

    private func smooth(_ x: Double) -> Double {
        let t = min(max(x, 0), 1)
        return t * t * (3 - 2 * t)
    }

    private func drawMark(context: inout GraphicsContext, geom: LeashClipGeom, tint: Color, filled: Bool, seated: CGFloat) {
        if filled {
            context.fill(Path(ellipseIn: geom.ring), with: .color(tint))
        } else {
            var ring = Path()
            ring.addEllipse(in: geom.ring.insetBy(dx: geom.line / 2, dy: geom.line / 2))
            context.stroke(ring, with: .color(tint), style: StrokeStyle(lineWidth: geom.line, lineCap: .round))
        }
        drawStrap(context: &context, geom: geom, tint: tint, seated: seated)
    }

    private func drawArc(context: inout GraphicsContext, geom: LeashClipGeom, tint: Color, start: Angle, sweep: Angle) {
        var arc = Path()
        arc.addArc(center: geom.center, radius: geom.radius, startAngle: start, endAngle: start + sweep, clockwise: false)
        context.stroke(arc, with: .color(tint), style: StrokeStyle(lineWidth: geom.line, lineCap: .round))
    }

    private func drawStrap(context: inout GraphicsContext, geom: LeashClipGeom, tint: Color, seated: CGFloat) {
        let retract = (1 - seated) * geom.strap.width * 0.9
        let rect = geom.strap.offsetBy(dx: retract, dy: 0)
        context.fill(
            Path(roundedRect: rect, cornerRadius: rect.height / 2),
            with: .color(tint)
        )
    }
}

struct LeashWorkBars: View {
    var tint: Color = LeashPaint.moss

    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 30.0)) { timeline in
            let t = timeline.date.timeIntervalSinceReferenceDate
            HStack(alignment: .center, spacing: LeashSpace.xs) {
                ForEach(0..<3, id: \.self) { i in
                    let wave = 0.5 + 0.5 * sin(t * 3.1 + Double(i) * 0.95)
                    Capsule()
                        .fill(tint.opacity(0.35 + 0.65 * wave))
                        .frame(width: 8 + 22 * wave, height: 3)
                }
            }
        }
        .accessibilityHidden(true)
    }
}

struct LeashIndeterminate: View {
    var tint: Color = LeashPaint.moss

    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 30.0)) { timeline in
            GeometryReader { geo in
                let w = geo.size.width
                let h = max(geo.size.height, 2)
                let t = timeline.date.timeIntervalSinceReferenceDate
                let cycle = t.truncatingRemainder(dividingBy: LeashMotion.trackPeriod) / LeashMotion.trackPeriod
                let bar = max(28, w * 0.34)
                let x = (w + bar) * cycle - bar
                Capsule()
                    .fill(tint)
                    .frame(width: bar, height: h)
                    .offset(x: x)
            }
        }
        .clipShape(Capsule())
        .background(tint.opacity(0.14), in: Capsule())
        .accessibilityHidden(true)
    }
}

struct LeashIdlePanel: View {
    var connecting: Bool
    var title: String
    var detail: String? = nil
    var compact: Bool = false

    var body: some View {
        VStack(spacing: compact ? LeashSpace.md : LeashSpace.lg) {
            LeashLoader(
                mode: connecting ? .fasten : .rest(filled: false),
                size: compact ? .panel : .hero,
                tint: connecting ? LeashPaint.moss : LeashPaint.muted
            )
            Text(title)
                .font(LeashType.empty)
                .foregroundStyle(LeashPaint.ink)
            if let detail, !detail.isEmpty {
                Text(detail)
                    .font(LeashType.body)
                    .foregroundStyle(LeashPaint.muted)
            }
        }
        .frame(maxWidth: .infinity, minHeight: compact ? LeashSpace.inspectorFloor : LeashSpace.emptyFloor)
        .padding(compact ? LeashSpace.xl : LeashSpace.empty)
    }
}
