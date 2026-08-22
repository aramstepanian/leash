import AppKit
import SwiftUI

struct LeashMark: View {
    var filled: Bool = false
    var tint: Color = LeashPaint.ink
    var size: CGFloat = LeashSpace.mark

    var body: some View {
        let line = max(LeashSpace.Mark.minStroke, size * LeashSpace.Mark.stroke)
        let circle = size * LeashSpace.Mark.circle
        let strap = size * LeashSpace.Mark.strap
        let gap = line * LeashSpace.Mark.gap
        HStack(spacing: gap) {
            ZStack {
                Circle().strokeBorder(tint, lineWidth: line)
                if filled {
                    Circle()
                        .fill(tint)
                        .padding(line * LeashSpace.Mark.fillPad)
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
        Text("\(LeashCopy.app)  \(LeashCopy.buildMark)")
            .font(LeashType.kicker)
            .tracking(LeashType.Track.wordmark)
            .textCase(.uppercase)
            .foregroundStyle(LeashPaint.muted)
    }
}

struct LeashKicker: View {
    var text: String

    var body: some View {
        Text(text)
            .font(LeashType.kicker)
            .tracking(LeashType.Track.kicker)
            .textCase(.uppercase)
            .foregroundStyle(LeashPaint.muted)
    }
}

struct KeyHint: View {
    var keys: String
    var on: LeashHint = .paper

    var body: some View {
        Text(keys)
            .font(LeashType.hint)
            .foregroundStyle(foreground)
            .padding(.horizontal, LeashSpace.hintX)
            .padding(.vertical, LeashSpace.xxs)
            .background(background, in: RoundedRectangle(cornerRadius: LeashSpace.radiusChip, style: .continuous))
    }

    private var foreground: Color {
        switch on {
        case .paper: return LeashPaint.muted
        case .ink: return LeashPaint.paper.opacity(LeashPaint.Opacity.hintInk)
        case .vermillion: return LeashPaint.bone.opacity(LeashPaint.Opacity.hintKill)
        }
    }

    private var background: Color {
        switch on {
        case .paper: return LeashPaint.faint
        case .ink: return LeashPaint.paper.opacity(LeashPaint.Opacity.hintOnInk)
        case .vermillion: return LeashPaint.bone.opacity(LeashPaint.Opacity.hintOnKill)
        }
    }
}

struct LeashChip: View {
    var title: String
    var tint: Color
    var on: Bool = true

    var body: some View {
        Text(title)
            .font(LeashType.chip)
            .tracking(LeashType.Track.chip)
            .foregroundStyle(on ? tint : LeashPaint.muted)
            .padding(.horizontal, LeashSpace.md)
            .padding(.vertical, LeashSpace.chipY)
            .background(
                (on ? tint : LeashPaint.ink).opacity(on ? LeashPaint.Opacity.chipOn : LeashPaint.Opacity.chipOff),
                in: Capsule()
            )
    }
}

struct KindChip: View {
    var kind: LeashKind

    var body: some View {
        LeashChip(title: kind.label, tint: kind.color)
    }
}

struct LeashPhaseLights: View {
    var phase: String
    var waiting: Int

    var body: some View {
        HStack(spacing: LeashSpace.sm) {
            ForEach(MissionPhase.lights) { light in
                LeashChip(title: light.label, tint: light.tint, on: light.isLit(phase: phase, waiting: waiting))
            }
            if MissionPhase(phase) == .failed {
                LeashChip(title: MissionPhase.failed.label, tint: MissionPhase.failed.tint)
            }
        }
    }
}

struct Hairline: View {
    var body: some View {
        Rectangle()
            .fill(LeashPaint.hairline)
            .frame(height: LeashSpace.hairline)
    }
}

struct LeashRail: View {
    var tint: Color
    var height: CGFloat? = nil

    var body: some View {
        RoundedRectangle(cornerRadius: LeashSpace.xxs, style: .continuous)
            .fill(tint)
            .frame(width: LeashSpace.rail, height: height)
    }
}

struct LeashWell<Content: View>: View {
    var rail: Color? = nil
    @ViewBuilder var content: () -> Content

    var body: some View {
        HStack(alignment: .top, spacing: 0) {
            if let rail {
                rail.frame(width: LeashSpace.rail)
            }
            content()
        }
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous)
                .strokeBorder(LeashPaint.hairline, lineWidth: LeashSpace.hairline)
        )
        .clipShape(RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
    }
}

struct LeashButton: View {
    var title: String
    var hint: String
    var action: LeashAction
    var size: LeashControlSize = .action
    var disabled: Bool = false
    var run: () -> Void

    var body: some View {
        Button(action: run) {
            HStack(spacing: action.compactPad ? LeashSpace.sm : LeashSpace.md) {
                Text(title)
                    .font(size == .control ? LeashType.bodyStrong : LeashType.row(weight: action.weight))
                KeyHint(keys: hint, on: action.hint)
            }
            .padding(.horizontal, size == .control ? LeashSpace.lg : (action.compactPad ? LeashSpace.compact : LeashSpace.xxl))
            .frame(height: size.height)
            .background(action.fill, in: RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous)
                    .strokeBorder(action.bordered ? LeashPaint.hairline : .clear, lineWidth: LeashSpace.hairline)
            )
            .foregroundStyle(action.ink)
        }
        .buttonStyle(.plain)
        .disabled(disabled)
    }
}

struct LeashField: View {
    var placeholder: String
    @Binding var text: String
    var onSubmit: (() -> Void)? = nil

    var body: some View {
        TextField(placeholder, text: $text)
            .textFieldStyle(.plain)
            .font(LeashType.body)
            .padding(.horizontal, LeashSpace.lg)
            .frame(height: LeashSpace.control)
            .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous))
            .onSubmit { onSubmit?() }
    }
}

struct LeashPathRow: View {
    var path: String
    var symbol: String = LeashSymbol.folder

    var body: some View {
        HStack(spacing: LeashSpace.sm) {
            Image(systemName: symbol)
                .font(LeashType.icon)
            Text(LeashFormat.compactPath(path))
                .font(LeashType.mono)
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .foregroundStyle(LeashPaint.muted)
    }
}

struct LeashMono: View {
    var text: String
    var strong: Bool = false
    var tint: Color = LeashPaint.muted

    var body: some View {
        Text(text)
            .font(strong ? LeashType.monoMedium : LeashType.mono)
            .foregroundStyle(tint)
            .lineLimit(1)
            .truncationMode(.middle)
    }
}

struct LeashMenuRow: View {
    var title: String
    var subtitle: String? = nil
    var symbol: String
    var disabled: Bool = false
    var action: () -> Void

    @State private var hover = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: LeashSpace.lg) {
                Image(systemName: symbol)
                    .font(LeashType.menuIcon)
                    .foregroundStyle(LeashPaint.muted)
                    .frame(width: LeashSpace.icon)
                VStack(alignment: .leading, spacing: LeashSpace.lead) {
                    Text(title)
                        .font(LeashType.row)
                        .foregroundStyle(LeashPaint.ink)
                    if let subtitle, !subtitle.isEmpty {
                        Text(subtitle)
                            .font(LeashType.caption)
                            .foregroundStyle(LeashPaint.muted)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, LeashSpace.md)
            .padding(.vertical, subtitle == nil ? LeashSpace.rowTight : LeashSpace.md)
            .background(
                hover && !disabled ? LeashPaint.faint : .clear,
                in: RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous)
            )
            .contentShape(RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous))
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? LeashPaint.Opacity.disabled : 1)
        .onHover { hover = $0 }
    }
}

struct LeashPendingRow: View {
    var pending: PendingApproval
    var queued: Bool
    var action: () -> Void

    var body: some View {
        let kind = LeashKind(pending.kind)
        Button(action: action) {
            HStack(spacing: LeashSpace.lg) {
                LeashRail(tint: kind.color, height: LeashSpace.status)
                VStack(alignment: .leading, spacing: LeashSpace.xxs) {
                    Text(LeashFormat.pendingHeadline(pending))
                        .font(LeashType.rowStrong)
                        .foregroundStyle(LeashPaint.ink)
                    Text(queued ? LeashCopy.queued : LeashCopy.waitingOnYou)
                        .font(LeashType.caption)
                        .foregroundStyle(kind.color)
                }
                Spacer()
                Image(systemName: LeashSymbol.chevron)
                    .font(LeashType.chevron)
                    .foregroundStyle(LeashPaint.muted)
            }
            .padding(.horizontal, LeashSpace.md)
            .padding(.vertical, LeashSpace.md)
            .background(
                kind.color.opacity(queued ? LeashPaint.Opacity.queued : LeashPaint.Opacity.pending),
                in: RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous)
            )
        }
        .buttonStyle(.plain)
    }
}

struct LeashStatusBadge: View {
    var tint: Color
    var filled: Bool

    var body: some View {
        ZStack {
            Circle()
                .fill(tint.opacity(LeashPaint.Opacity.statusHalo))
                .frame(width: LeashSpace.status, height: LeashSpace.status)
            LeashMark(filled: filled, tint: tint)
        }
    }
}

struct LeashTapeRow: View {
    var event: TimelineEvent
    var selected: Bool
    var action: () -> Void

    var body: some View {
        let kind = TapeKind(event.kind)
        Button(action: action) {
            HStack(spacing: LeashSpace.md) {
                Circle()
                    .fill(kind.color)
                    .frame(width: LeashSpace.dot, height: LeashSpace.dot)
                VStack(alignment: .leading, spacing: LeashSpace.lead) {
                    HStack {
                        Text(event.title)
                            .font(LeashType.bodyMedium)
                            .foregroundStyle(LeashPaint.ink)
                            .lineLimit(1)
                        Spacer()
                        if let ms = event.durationMs, ms > 0 {
                            Text(LeashFormat.duration(ms))
                                .font(LeashType.hint)
                                .foregroundStyle(LeashPaint.muted)
                        }
                    }
                    Text(TapeKind(event.kind).label.uppercased())
                        .font(LeashType.tape)
                        .tracking(LeashType.Track.tape)
                        .foregroundStyle(kind.color)
                }
            }
            .padding(.horizontal, LeashSpace.md)
            .padding(.vertical, LeashSpace.sm)
            .background(
                selected ? LeashPaint.faint : .clear,
                in: RoundedRectangle(cornerRadius: LeashSpace.radiusRow, style: .continuous)
            )
        }
        .buttonStyle(.plain)
    }
}

struct LeashInspector: View {
    var live: LiveCall

    var body: some View {
        let status = LiveStatus(live.status)
        LeashWell(rail: status.tint) {
            VStack(alignment: .leading, spacing: LeashSpace.sm) {
                HStack(spacing: LeashSpace.md) {
                    Text(LeashFormat.liveHeadline(live))
                        .font(LeashType.rowStrong)
                    Text(status.label)
                        .font(LeashType.chip)
                        .foregroundStyle(status.tint)
                    Spacer()
                    if let agent = live.agent, !agent.isEmpty {
                        LeashMono(text: agent)
                    }
                    if let ms = live.durationMs, ms > 0 {
                        Text(LeashFormat.duration(ms))
                            .font(LeashType.mono)
                            .foregroundStyle(LeashPaint.muted)
                    }
                }
                if status == .waiting, !live.detail.isEmpty {
                    Text(live.detail)
                        .font(LeashType.codeMedium)
                        .foregroundStyle(LeashPaint.code)
                        .textSelection(.enabled)
                        .lineLimit(4)
                }
                if let err = live.error, !err.isEmpty {
                    Text(err)
                        .font(LeashType.mono)
                        .foregroundStyle(LeashPaint.vermillion)
                        .lineLimit(3)
                }
            }
            .padding(LeashSpace.xl)
        }
    }
}

struct LeashCommandWell: View {
    var text: String
    var needles: [String]
    var accent: Color

    var body: some View {
        let body = Text(highlightedCommand(text, needles: needles, accent: accent))
            .textSelection(.enabled)
            .lineSpacing(LeashSpace.line)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, LeashSpace.xxl)
            .padding(.vertical, LeashSpace.xl)

        return LeashWell(rail: accent) {
            ViewThatFits(in: .vertical) {
                body
                ScrollView(.vertical, showsIndicators: false) {
                    body
                }
                .frame(maxHeight: LeashSpace.commandCeiling)
            }
        }
    }
}

struct LeashEmptyWell: View {
    var text: String
    var minHeight: CGFloat = LeashSpace.inspectorFloor

    var body: some View {
        Text(text)
            .font(LeashType.body)
            .foregroundStyle(LeashPaint.muted)
            .frame(maxWidth: .infinity, minHeight: minHeight, alignment: .leading)
            .padding(LeashSpace.xl)
            .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
    }
}

struct LeashCaughtUp: View {
    var body: some View {
        VStack(spacing: LeashSpace.lg) {
            LeashMark(filled: true, tint: LeashPaint.muted)
            Text(LeashCopy.caughtUp)
                .font(LeashType.empty)
                .foregroundStyle(LeashPaint.ink)
            Text(LeashCopy.noWaiting)
                .font(LeashType.body)
                .foregroundStyle(LeashPaint.muted)
        }
        .frame(maxWidth: .infinity, minHeight: LeashSpace.emptyFloor)
        .padding(LeashSpace.empty)
    }
}

struct LeashActionBar: View {
    var deciding: Bool
    var size: LeashControlSize = .action
    var kill: () -> Void
    var always: () -> Void
    var allow: () -> Void

    var body: some View {
        HStack(spacing: LeashSpace.md) {
            killButton
            if size == .action {
                Spacer(minLength: LeashSpace.md)
            }
            LeashButton(title: LeashCopy.always, hint: LeashCopy.hintCommandReturn, action: .always, size: size, disabled: deciding, run: always)
                .keyboardShortcut(.return, modifiers: [.command])
            LeashButton(title: LeashCopy.allow, hint: LeashCopy.hintReturn, action: .allow, size: size, disabled: deciding, run: allow)
                .keyboardShortcut(.defaultAction)
        }
        .opacity(deciding ? LeashPaint.Opacity.deciding : 1)
        .animation(LeashMotion.snap, value: deciding)
    }

    @ViewBuilder
    private var killButton: some View {
        if size == .action {
            LeashButton(title: LeashCopy.kill, hint: LeashCopy.hintEsc, action: .kill, size: size, disabled: deciding, run: kill)
                .keyboardShortcut(.cancelAction)
        } else {
            LeashButton(title: LeashCopy.kill, hint: LeashCopy.hintEsc, action: .kill, size: size, disabled: deciding, run: kill)
        }
    }
}

struct LeashGateBar: View {
    var pending: PendingApproval
    var deciding: Bool
    var kill: () -> Void
    var always: () -> Void
    var allow: () -> Void

    var body: some View {
        HStack(spacing: LeashSpace.md) {
            KindChip(kind: LeashKind(pending.kind))
            if !pending.reasons.isEmpty {
                Text(pending.reasons.joined(separator: LeashCopy.dot))
                    .font(LeashType.caption)
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(1)
            }
            Spacer(minLength: LeashSpace.md)
            LeashActionBar(deciding: deciding, size: .control, kill: kill, always: always, allow: allow)
        }
    }
}

struct LeashFailureBar: View {
    var failed: FailedCall
    var skip: () -> Void
    var retry: () -> Void

    var body: some View {
        HStack(spacing: LeashSpace.md) {
            VStack(alignment: .leading, spacing: LeashSpace.xxs) {
                Text(LeashFormat.failedHeadline(failed))
                    .font(LeashType.bodyStrong)
                    .foregroundStyle(LeashPaint.ink)
                    .lineLimit(1)
                Text(failed.error)
                    .font(LeashType.mono)
                    .foregroundStyle(LeashPaint.vermillion)
                    .lineLimit(2)
            }
            Spacer()
            LeashButton(title: LeashCopy.skip, hint: LeashCopy.hintSkip, action: .ghost, size: .control, run: skip)
                .keyboardShortcut(".", modifiers: [.command])
            LeashButton(title: LeashCopy.retry, hint: LeashCopy.hintRetry, action: .retry, size: .control, run: retry)
                .keyboardShortcut("r", modifiers: [.command])
        }
    }
}

struct LeashSteerBar: View {
    @Binding var text: String
    var canRewind: Bool
    var onSteer: () -> Void
    var onCut: () -> Void
    var onRewind: () -> Void

    var body: some View {
        HStack(spacing: LeashSpace.md) {
            LeashField(placeholder: LeashCopy.steerPrompt, text: $text, onSubmit: onSteer)
            LeashButton(title: LeashCopy.steer, hint: LeashCopy.hintSteer, action: .ghost, size: .control, run: onSteer)
                .keyboardShortcut("l", modifiers: [.command])
            LeashButton(title: LeashCopy.cut, hint: LeashCopy.hintEsc, action: .kill, size: .control, run: onCut)
            LeashButton(title: LeashCopy.rewind, hint: LeashCopy.hintUndo, action: .ghost, size: .control, disabled: !canRewind, run: onRewind)
                .keyboardShortcut("z", modifiers: [.command])
        }
    }
}

struct LeashTapeList: View {
    var events: [TimelineEvent]
    @Binding var selectedID: String?

    var body: some View {
        VStack(alignment: .leading, spacing: LeashSpace.sm) {
            LeashKicker(text: LeashCopy.tape)
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: LeashSpace.xxs) {
                        ForEach(events) { event in
                            LeashTapeRow(event: event, selected: selectedID == event.id) {
                                selectedID = event.id
                            }
                            .id(event.id)
                        }
                    }
                }
                .onChange(of: events.last?.id) { _, id in
                    if let id { proxy.scrollTo(id, anchor: .bottom) }
                }
            }
        }
    }
}

struct LeashScrub: View {
    var event: TimelineEvent?
    var burst: BurstInfo?

    var body: some View {
        VStack(alignment: .leading, spacing: LeashSpace.md) {
            LeashKicker(text: LeashCopy.result)
            if let event {
                eventHeader(event)
            }
            paths(from: event, burst: burst)
            Spacer(minLength: 0)
        }
        .padding(LeashSpace.xl)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
    }

    private func eventHeader(_ event: TimelineEvent) -> some View {
        HStack {
            Text(event.title)
                .font(LeashType.rowStrong)
                .lineLimit(2)
            Spacer()
            if let agent = event.agent, !agent.isEmpty {
                LeashMono(text: agent)
            }
        }
    }

    @ViewBuilder
    private func paths(from event: TimelineEvent?, burst: BurstInfo?) -> some View {
        let list = pathList(event: event, burst: burst)
        if list.isEmpty {
            Text(event?.detail ?? LeashCopy.emptyTape)
                .font(LeashType.body)
                .foregroundStyle(LeashPaint.muted)
        } else {
            ForEach(list.prefix(LeashLayout.previewFiles), id: \.self) { path in
                LeashMono(text: LeashFormat.compactPath(path))
            }
            if list.count > LeashLayout.previewFiles {
                Text(LeashCopy.andMore(list.count - LeashLayout.previewFiles))
                    .font(LeashType.caption)
                    .foregroundStyle(LeashPaint.muted)
            }
        }
        if let err = event?.error, !err.isEmpty {
            Text(err)
                .font(LeashType.code)
                .foregroundStyle(LeashPaint.vermillion)
                .textSelection(.enabled)
        }
    }

    private func pathList(event: TimelineEvent?, burst: BurstInfo?) -> [String] {
        if let paths = event?.paths, !paths.isEmpty {
            return paths
        }
        return burst?.files ?? []
    }
}

struct LeashResultWell: View {
    var burst: BurstInfo

    var body: some View {
        LeashWell(rail: LeashPaint.amber) {
            VStack(alignment: .leading, spacing: LeashSpace.sm) {
                HStack {
                    Text(LeashCopy.files(burst.fileCount))
                        .font(LeashType.rowStrong)
                    Text(LeashCopy.readyRewind)
                        .font(LeashType.chip)
                        .foregroundStyle(LeashPaint.amber)
                    Spacer()
                    if let root = burst.root, !root.isEmpty {
                        LeashMono(text: LeashFormat.folderName(root))
                    }
                }
                ForEach(burst.files.prefix(LeashLayout.previewFiles), id: \.self) { path in
                    LeashMono(text: LeashFormat.compactPath(path), tint: LeashPaint.code)
                }
                if burst.files.count > LeashLayout.previewFiles {
                    Text(LeashCopy.andMore(burst.files.count - LeashLayout.previewFiles))
                        .font(LeashType.caption)
                        .foregroundStyle(LeashPaint.muted)
                }
            }
            .padding(LeashSpace.xl)
        }
    }
}

struct LeashRemovableRow: View {
    var title: String
    var subtitle: String? = nil
    var symbol: String
    var remove: () -> Void

    @State private var hover = false

    var body: some View {
        HStack(spacing: LeashSpace.lg) {
            Image(systemName: symbol)
                .font(LeashType.menuIcon)
                .foregroundStyle(LeashPaint.muted)
                .frame(width: LeashSpace.icon)
            VStack(alignment: .leading, spacing: LeashSpace.lead) {
                Text(title)
                    .font(LeashType.row)
                    .foregroundStyle(LeashPaint.ink)
                    .lineLimit(1)
                    .truncationMode(.middle)
                if let subtitle, !subtitle.isEmpty {
                    Text(subtitle)
                        .font(LeashType.caption)
                        .foregroundStyle(LeashPaint.muted)
                        .lineLimit(1)
                        .truncationMode(.middle)
                }
            }
            Spacer(minLength: 0)
            Button(action: remove) {
                Image(systemName: LeashSymbol.alwaysList)
                    .font(LeashType.icon)
                    .foregroundStyle(LeashPaint.muted)
            }
            .buttonStyle(.plain)
        }
        .padding(.horizontal, LeashSpace.md)
        .padding(.vertical, subtitle == nil ? LeashSpace.rowTight : LeashSpace.md)
        .background(
            hover ? LeashPaint.faint : .clear,
            in: RoundedRectangle(cornerRadius: LeashSpace.radiusControl, style: .continuous)
        )
        .onHover { hover = $0 }
    }
}

struct LeashNotice: View {
    var text: String

    var body: some View {
        Text(text)
            .font(LeashType.captionMedium)
            .foregroundStyle(LeashPaint.ink.opacity(LeashPaint.Opacity.kindOther))
            .padding(.horizontal, LeashSpace.md)
            .padding(.top, LeashSpace.sm)
            .padding(.bottom, LeashSpace.xxs)
    }
}

struct LeashStatusHeader: View {
    var title: String
    var detail: String
    var tint: Color
    var filled: Bool
    var running: Bool = false
    @State private var pulse = false

    var body: some View {
        HStack(alignment: .center, spacing: LeashSpace.lg) {
            LeashStatusBadge(tint: tint, filled: filled)
                .scaleEffect(running && pulse ? 1.12 : 1)
                .opacity(running && pulse ? LeashPaint.Opacity.pulse : 1)
                .animation(running ? LeashMotion.pulse : LeashMotion.settle, value: pulse)
            VStack(alignment: .leading, spacing: LeashSpace.xxs) {
                Text(title)
                    .font(LeashType.rowStrong)
                    .foregroundStyle(LeashPaint.ink)
                Text(detail)
                    .font(LeashType.mono)
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            Spacer(minLength: LeashSpace.md)
            LeashWordmark()
        }
        .padding(.horizontal, LeashSpace.sm)
        .padding(.top, LeashSpace.xs)
        .padding(.bottom, LeashSpace.xxs)
        .onAppear { pulse = running }
        .onChange(of: running) { _, isRunning in
            pulse = isRunning
        }
    }
}

struct LeashRunningPulse: View {
    @State private var beat = false
    @State private var hop = 0

    var body: some View {
        VStack(spacing: LeashSpace.lg) {
            LeashMark(filled: true, tint: LeashPaint.moss, size: 28)
                .scaleEffect(beat ? 1.14 : 0.88)
                .opacity(beat ? 1 : 0.55)
                .animation(LeashMotion.pulse, value: beat)
            HStack(spacing: LeashSpace.sm) {
                ForEach(0..<3, id: \.self) { i in
                    Circle()
                        .fill(LeashPaint.moss)
                        .frame(width: LeashSpace.dot, height: LeashSpace.dot)
                        .scaleEffect(hop == i ? 1.35 : 0.7)
                        .opacity(hop == i ? 1 : 0.28)
                }
            }
            .animation(LeashMotion.snap, value: hop)
            Text(LeashCopy.working)
                .font(LeashType.kicker)
                .tracking(LeashType.Track.kicker)
                .textCase(.uppercase)
                .foregroundStyle(LeashPaint.moss)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, LeashSpace.section)
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
        .onAppear { beat = true }
        .onReceive(Timer.publish(every: 0.28, on: .main, in: .common).autoconnect()) { _ in
            hop = (hop + 1) % 3
        }
    }
}

struct LeashReplyWell: View {
    var text: String
    var failed: Bool = false

    var body: some View {
        ScrollView {
            Text(text)
                .font(LeashType.body)
                .foregroundStyle(failed ? LeashPaint.vermillion : LeashPaint.ink)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .frame(maxHeight: LeashSpace.commandCeiling)
        .padding(LeashSpace.xl)
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: LeashSpace.radiusWell, style: .continuous)
                .strokeBorder(LeashPaint.hairline, lineWidth: LeashSpace.hairline)
        )
    }
}
