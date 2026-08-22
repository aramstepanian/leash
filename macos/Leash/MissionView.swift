import AppKit
import SwiftUI

struct MissionView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            Hairline().padding(.vertical, 10)
            inspector
            if let pending = app.state.pending {
                gate(pending)
                    .padding(.top, 12)
            } else if let failed = app.state.mission?.failed {
                failure(failed)
                    .padding(.top, 12)
            }
            steerRow
                .padding(.top, 12)
            Hairline().padding(.vertical, 10)
            HStack(alignment: .top, spacing: 14) {
                timeline
                    .frame(width: 280)
                detail
            }
            .frame(minHeight: 180)
        }
        .padding(18)
        .frame(width: 720, height: 520)
        .leashWindowFill()
        .background(WindowAccess(configure: LeashChrome.mission))
        .onAppear { app.start() }
        .onExitCommand {
            Task { await app.interrupt() }
        }
    }

    private var mission: MissionInfo? { app.state.mission }
    private var phase: String { mission?.phase ?? "idle" }

    private var header: some View {
        HStack(alignment: .center, spacing: 10) {
            LeashMark(filled: phase == "act" || phase == "failed" || app.state.waitingCount > 0, tint: phaseTint, size: 14)
            Text("Mission Control")
                .font(.system(size: 10, weight: .semibold))
                .tracking(1.6)
                .textCase(.uppercase)
                .foregroundStyle(LeashPaint.muted)
            Text(mission?.title ?? "Idle")
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(LeashPaint.ink)
                .lineLimit(1)
            if let agent = mission?.agent, !agent.isEmpty {
                Text(agent)
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(LeashPaint.muted)
            }
            Spacer(minLength: 8)
            phaseLights
        }
    }

    private var phaseLights: some View {
        HStack(spacing: 6) {
            phaseChip("Plan", on: phase == "plan")
            phaseChip("Act", on: phase == "act" || app.state.waitingCount > 0)
            phaseChip("Review", on: phase == "review")
            if phase == "failed" {
                phaseChip("Fail", on: true, tint: LeashPaint.vermillion)
            }
        }
    }

    private func phaseChip(_ title: String, on: Bool, tint: Color = LeashPaint.moss) -> some View {
        Text(title)
            .font(.system(size: 10, weight: .semibold))
            .tracking(0.4)
            .foregroundStyle(on ? tint : LeashPaint.muted)
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background((on ? tint : LeashPaint.ink).opacity(on ? 0.14 : 0.05), in: Capsule())
    }

    private var phaseTint: Color {
        if phase == "failed" || app.state.waitingCount > 0 { return LeashPaint.vermillion }
        if phase == "act" { return LeashPaint.moss }
        if phase == "plan" { return LeashPaint.steel }
        return LeashPaint.muted
    }

    private var inspector: some View {
        VStack(alignment: .leading, spacing: 6) {
            if let goal = mission?.goal, !goal.isEmpty {
                Text(goal)
                    .font(.system(size: 12))
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(2)
            }
            if let live = mission?.live {
                liveCard(live)
            } else if let pending = app.state.pending {
                liveCard(LiveCall(
                    tool: pending.tool,
                    detail: pending.detail,
                    agent: pending.agent,
                    root: pending.root,
                    started: nil,
                    status: "waiting",
                    durationMs: nil,
                    result: nil,
                    error: nil
                ))
            } else {
                Text("No tool in flight.")
                    .font(.system(size: 12))
                    .foregroundStyle(LeashPaint.muted)
                    .frame(maxWidth: .infinity, minHeight: 64, alignment: .leading)
                    .padding(12)
                    .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
            }
        }
    }

    private func liveCard(_ live: LiveCall) -> some View {
        let accent = live.status == "error" || live.status == "waiting" ? LeashPaint.vermillion : LeashPaint.moss
        return HStack(alignment: .top, spacing: 0) {
            accent.frame(width: 2)
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 8) {
                    Text(live.agent.map { "\($0) · \(live.tool)" } ?? live.tool)
                        .font(.system(size: 12, weight: .semibold, design: .monospaced))
                    Text(live.status)
                        .font(.system(size: 10, weight: .semibold))
                        .foregroundStyle(accent)
                    Spacer()
                    if let ms = live.durationMs, ms > 0 {
                        Text(formatMs(ms))
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(LeashPaint.muted)
                    }
                }
                Text(live.detail)
                    .font(.system(size: 12.5, weight: .medium, design: .monospaced))
                    .foregroundStyle(LeashPaint.ink.opacity(0.9))
                    .textSelection(.enabled)
                    .lineLimit(4)
                if let err = live.error, !err.isEmpty {
                    Text(err)
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(LeashPaint.vermillion)
                        .lineLimit(3)
                } else if let result = live.result, !result.isEmpty {
                    Text(result)
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(LeashPaint.muted)
                        .lineLimit(3)
                }
            }
            .padding(12)
        }
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .strokeBorder(LeashPaint.hairline, lineWidth: 1)
        )
    }

    private func gate(_ pending: PendingApproval) -> some View {
        let kind = LeashKind(pending.kind)
        return HStack(spacing: 8) {
            KindChip(kind: kind)
            Text(pending.reasons.joined(separator: " · "))
                .font(.system(size: 11))
                .foregroundStyle(LeashPaint.muted)
                .lineLimit(1)
            Spacer(minLength: 8)
            compactButton("Kill", hint: "esc", fill: LeashPaint.vermillion, ink: LeashPaint.bone) {
                Task { await app.decide("kill") }
            }
            compactButton("Always", hint: "⌘↩", fill: LeashPaint.faint, ink: LeashPaint.ink) {
                Task { await app.decide("always") }
            }
            .keyboardShortcut(.return, modifiers: [.command])
            compactButton("Allow", hint: "↩", fill: LeashPaint.ink, ink: LeashPaint.paper) {
                Task { await app.decide("allow") }
            }
            .keyboardShortcut(.defaultAction)
        }
        .opacity(app.deciding ? 0.55 : 1)
    }

    private func failure(_ failed: FailedCall) -> some View {
        HStack(spacing: 8) {
            Text(failed.error)
                .font(.system(size: 11, design: .monospaced))
                .foregroundStyle(LeashPaint.vermillion)
                .lineLimit(2)
            Spacer()
            compactButton("Skip", hint: "⌘.", fill: LeashPaint.faint, ink: LeashPaint.ink) {
                Task { await app.skipFail() }
            }
            .keyboardShortcut(".", modifiers: [.command])
            compactButton("Retry", hint: "⌘R", fill: LeashPaint.ink, ink: LeashPaint.paper) {
                Task { await app.retry() }
            }
            .keyboardShortcut("r", modifiers: [.command])
        }
    }

    private var steerRow: some View {
        HStack(spacing: 8) {
            TextField("Steer the agent…", text: $app.steerDraft)
                .textFieldStyle(.plain)
                .font(.system(size: 12))
                .padding(.horizontal, 10)
                .frame(height: 32)
                .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            compactButton("Steer", hint: "⌘L", fill: LeashPaint.faint, ink: LeashPaint.ink) {
                Task { await app.steer() }
            }
            .keyboardShortcut("l", modifiers: [.command])
            compactButton("Cut", hint: "esc", fill: LeashPaint.vermillion, ink: LeashPaint.bone) {
                Task { await app.interrupt() }
            }
            compactButton("Rewind", hint: "⌘Z", fill: LeashPaint.faint, ink: LeashPaint.ink) {
                Task { await app.undo() }
            }
            .keyboardShortcut("z", modifiers: [.command])
            .disabled(app.state.burst == nil)
        }
    }

    private var timeline: some View {
        let events = mission?.timeline ?? []
        return VStack(alignment: .leading, spacing: 6) {
            Text("Timeline")
                .font(.system(size: 10, weight: .semibold))
                .tracking(1.4)
                .textCase(.uppercase)
                .foregroundStyle(LeashPaint.muted)
            ScrollViewReader { proxy in
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 2) {
                        ForEach(events) { ev in
                            timelineRow(ev)
                                .id(ev.id)
                        }
                    }
                }
                .onChange(of: events.last?.id) { _, id in
                    if let id { proxy.scrollTo(id, anchor: .bottom) }
                }
            }
        }
    }

    private func timelineRow(_ ev: TimelineEvent) -> some View {
        let on = app.selectedEventID == ev.id
        return Button {
            app.selectedEventID = ev.id
        } label: {
            HStack(spacing: 8) {
                Circle()
                    .fill(kindColor(ev.kind))
                    .frame(width: 6, height: 6)
                VStack(alignment: .leading, spacing: 1) {
                    HStack {
                        Text(ev.title)
                            .font(.system(size: 12, weight: .medium))
                            .foregroundStyle(LeashPaint.ink)
                            .lineLimit(1)
                        Spacer()
                        if let ms = ev.durationMs, ms > 0 {
                            Text(formatMs(ms))
                                .font(.system(size: 10, design: .monospaced))
                                .foregroundStyle(LeashPaint.muted)
                        }
                    }
                    Text(ev.kind.uppercased())
                        .font(.system(size: 9, weight: .semibold))
                        .tracking(0.6)
                        .foregroundStyle(kindColor(ev.kind))
                }
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 6)
            .background(on ? LeashPaint.faint : .clear, in: RoundedRectangle(cornerRadius: 6, style: .continuous))
        }
        .buttonStyle(.plain)
    }

    private var detail: some View {
        let ev = (mission?.timeline ?? []).first { $0.id == app.selectedEventID } ?? mission?.timeline.last
        return VStack(alignment: .leading, spacing: 8) {
            Text("Scrub")
                .font(.system(size: 10, weight: .semibold))
                .tracking(1.4)
                .textCase(.uppercase)
                .foregroundStyle(LeashPaint.muted)
            if let ev {
                HStack {
                    Text(ev.kind.capitalized)
                        .font(.system(size: 13, weight: .semibold))
                    if let agent = ev.agent, !agent.isEmpty {
                        Text(agent)
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(LeashPaint.muted)
                    }
                    if let tool = ev.tool, !tool.isEmpty {
                        Text(tool)
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(LeashPaint.muted)
                    }
                    Spacer()
                    if let result = ev.result, !result.isEmpty {
                        Text(result)
                            .font(.system(size: 10, weight: .semibold))
                            .foregroundStyle(result == "error" || result == "deny" ? LeashPaint.vermillion : LeashPaint.moss)
                    }
                }
                if let detail = ev.detail, !detail.isEmpty {
                    Text(detail)
                        .font(.system(size: 12, design: .monospaced))
                        .foregroundStyle(LeashPaint.ink.opacity(0.9))
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                if let err = ev.error, !err.isEmpty {
                    Text(err)
                        .font(.system(size: 12, design: .monospaced))
                        .foregroundStyle(LeashPaint.vermillion)
                        .textSelection(.enabled)
                }
                if let paths = ev.paths, !paths.isEmpty {
                    ForEach(paths.prefix(8), id: \.self) { p in
                        Text(compactPath(p))
                            .font(.system(size: 11, design: .monospaced))
                            .foregroundStyle(LeashPaint.muted)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
                if let root = ev.root, !root.isEmpty {
                    Text(compactPath(root))
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundStyle(LeashPaint.muted)
                }
            } else {
                Text("Nothing on the tape yet.")
                    .font(.system(size: 12))
                    .foregroundStyle(LeashPaint.muted)
            }
            Spacer(minLength: 0)
        }
        .padding(12)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .background(LeashPaint.well, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
    }

    private func compactButton(_ title: String, hint: String, fill: Color, ink: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Text(title)
                    .font(.system(size: 12, weight: .semibold))
                KeyHint(keys: hint, on: fill == LeashPaint.vermillion ? .vermillion : fill == LeashPaint.ink ? .ink : .paper)
            }
            .padding(.horizontal, 10)
            .frame(height: 32)
            .background(fill, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            .foregroundStyle(ink)
        }
        .buttonStyle(.plain)
    }

    private func kindColor(_ kind: String) -> Color {
        switch kind {
        case "plan": return LeashPaint.steel
        case "thought", "steer": return LeashPaint.muted
        case "tool", "undo": return LeashPaint.moss
        case "diff": return LeashPaint.steel
        case "gate": return LeashPaint.amber
        case "interrupt", "error": return LeashPaint.vermillion
        default: return LeashPaint.muted
        }
    }

    private func formatMs(_ ms: Int) -> String {
        if ms < 1000 { return "\(ms)ms" }
        return String(format: "%.1fs", Double(ms) / 1000)
    }
}
