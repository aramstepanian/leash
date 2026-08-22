import AppKit
import SwiftUI

struct ApprovalView: View {
    @EnvironmentObject private var app: AppModel
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        Group {
            if let pending = app.state.pending {
                panel(pending)
            } else {
                LeashCaughtUp()
            }
        }
        .frame(width: LeashLayout.approvalWidth)
        .leashChrome(LeashChrome.approval)
        .onAppear {
            app.start()
            polishWindow()
        }
        .onReceive(NotificationCenter.default.publisher(for: NSWindow.didBecomeKeyNotification)) { _ in
            polishWindow()
        }
        .onChange(of: app.state.pending?.id) { _, new in
            if new == nil { dismiss() }
        }
        .onExitCommand {
            Task { await app.decide("kill") }
        }
    }

    private func panel(_ pending: PendingApproval) -> some View {
        let kind = LeashKind(pending.kind)
        return VStack(alignment: .leading, spacing: 0) {
            header(pending, kind: kind)
            titleBlock(pending)
                .padding(.top, LeashSpace.panel)
            LeashCommandWell(text: pending.detail, needles: pending.reasons, accent: kind.color)
                .padding(.top, LeashSpace.section)
            if let folder = pending.root ?? pending.cwd, !folder.isEmpty {
                LeashPathRow(path: folder)
                    .padding(.top, LeashSpace.lg)
            }
            if let note = LeashFormat.moreWaiting(total: app.state.waitingCount) {
                Text(note)
                    .font(LeashType.caption)
                    .foregroundStyle(LeashPaint.muted)
                    .padding(.top, LeashSpace.md)
            }
            LeashActionBar(
                deciding: app.deciding,
                kill: { Task { await app.decide("kill") } },
                always: { Task { await app.decide("always") } },
                allow: { Task { await app.decide("allow") } }
            )
            .padding(.top, LeashSpace.sheet)
        }
        .padding(.horizontal, LeashSpace.sheet)
        .padding(.vertical, LeashSpace.panel)
    }

    private func header(_ pending: PendingApproval, kind: LeashKind) -> some View {
        let agent = pending.agent?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return HStack(spacing: LeashSpace.md) {
            LeashMark(filled: true, tint: kind.color)
            LeashWordmark()
            Spacer()
            if !agent.isEmpty {
                LeashMono(text: agent)
            }
            KindChip(kind: kind)
        }
    }

    private func titleBlock(_ pending: PendingApproval) -> some View {
        VStack(alignment: .leading, spacing: LeashSpace.sm) {
            Text(pending.title)
                .font(LeashType.display)
                .tracking(LeashType.Track.display)
                .foregroundStyle(LeashPaint.ink)
            if !pending.reasons.isEmpty {
                Text(pending.reasons.joined(separator: LeashCopy.reasons))
                    .font(LeashType.row)
                    .foregroundStyle(LeashPaint.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func polishWindow() {
        for window in NSApp.windows where window.identifier?.rawValue == LeashLayout.approvalID || window.title == LeashCopy.app {
            LeashChrome.approval(window)
        }
    }
}
