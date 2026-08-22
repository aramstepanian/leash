import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            statusHeader
            Hairline()
                .padding(.top, 12)
                .padding(.bottom, 6)

            ForEach(Array(app.state.allPending.enumerated()), id: \.element.id) { i, pending in
                pendingRow(pending, queued: i > 0)
                    .padding(.bottom, 6)
            }
            if !app.state.allPending.isEmpty {
                Hairline()
                    .padding(.bottom, 6)
            }

            MenuRow(title: "Mission Control", subtitle: missionSubtitle, symbol: "rectangle.3.group") {
                app.openMission()
            }
            MenuRow(title: "Watch folders", subtitle: watchSubtitle, symbol: "folder") {
                app.pickFolder()
            }
            MenuRow(
                title: "Undo last burst",
                subtitle: undoSubtitle,
                symbol: "arrow.uturn.backward",
                disabled: app.state.burst == nil
            ) {
                Task { await app.undo() }
            }
            MenuRow(title: "Install hooks", subtitle: "Cursor · Claude · Codex · OpenCode", symbol: "square.and.arrow.down") {
                Task { await app.installHooks() }
            }

            if let notice = app.notice {
                Text(notice)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(LeashPaint.ink.opacity(0.7))
                    .padding(.horizontal, 8)
                    .padding(.top, 6)
                    .padding(.bottom, 2)
            }

            Hairline()
                .padding(.vertical, 6)

            MenuRow(title: "Quit Leash", symbol: "power") {
                NSApplication.shared.terminate(nil)
            }
        }
        .padding(10)
        .frame(width: 300)
        .leashWindowFill()
        .background(WindowAccess(configure: LeashChrome.menu))
        .onAppear { app.start() }
    }

    private var statusHeader: some View {
        HStack(alignment: .center, spacing: 10) {
            ZStack {
                Circle()
                    .fill(statusTint.opacity(0.14))
                    .frame(width: 28, height: 28)
                LeashMark(filled: app.state.waitingCount > 0 || app.state.status == "watching", tint: statusTint, size: 14)
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(statusTitle)
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(LeashPaint.ink)
                Text(statusDetail)
                    .font(.system(size: 11, design: .monospaced))
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            Spacer(minLength: 8)
            LeashWordmark()
        }
        .padding(.horizontal, 6)
        .padding(.top, 4)
        .padding(.bottom, 2)
    }

    private func pendingRow(_ pending: PendingApproval, queued: Bool) -> some View {
        let kind = LeashKind(pending.kind)
        return Button {
            ApprovalHUD.shared.show(model: app)
        } label: {
            HStack(spacing: 10) {
                RoundedRectangle(cornerRadius: 2, style: .continuous)
                    .fill(kind.color)
                    .frame(width: 2, height: 28)
                VStack(alignment: .leading, spacing: 2) {
                    Text(pendingHeadline(pending))
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(LeashPaint.ink)
                    Text(queued ? "Queued" : "Waiting on you")
                        .font(.system(size: 11))
                        .foregroundStyle(kind.color)
                }
                Spacer()
                Image(systemName: "chevron.right")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(LeashPaint.muted)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 8)
            .background(kind.color.opacity(queued ? 0.06 : 0.10), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
        }
        .buttonStyle(.plain)
    }

    private func pendingHeadline(_ pending: PendingApproval) -> String {
        let agent = pending.agent?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if !agent.isEmpty && !pending.tool.isEmpty {
            return "\(agent) · \(pending.tool)"
        }
        if !agent.isEmpty {
            return agent
        }
        return pending.title
    }

    private var statusTitle: String {
        if app.state.waitingCount > 1 { return "Needs you · \(app.state.waitingCount)" }
        if app.state.waitingCount == 1 { return "Needs you" }
        if app.daemonError != nil { return "Offline" }
        switch app.state.status {
        case "watching": return "Watching"
        case "waiting": return "Needs you"
        case "idle": return "Idle"
        default: return "Offline"
        }
    }

    private var statusDetail: String {
        if let pending = app.state.pending {
            return pendingHeadline(pending)
        }
        if let err = app.daemonError {
            return err
        }
        let folders = app.state.folders
        if folders.count > 1 {
            return "\(folders.count) folders"
        }
        if let root = folders.first, !root.isEmpty {
            return compactPath(root)
        }
        return "Pick a folder to protect"
    }

    private var statusTint: Color {
        if app.state.waitingCount > 0 { return LeashPaint.vermillion }
        if app.daemonError != nil { return LeashPaint.muted }
        if app.state.status == "watching" { return Color(nsColor: NSColor(srgbRed: 0.310, green: 0.545, blue: 0.427, alpha: 1)) }
        return LeashPaint.muted
    }

    private var missionSubtitle: String {
        let phase = app.state.mission?.phase ?? "idle"
        if let title = app.state.mission?.title, !title.isEmpty, phase != "idle" {
            return "\(phase) · \(title)"
        }
        return "Plan · act · review"
    }

    private var watchSubtitle: String {
        let folders = app.state.folders
        if folders.isEmpty {
            return "Choose the project folders"
        }
        return folders.map(compactPath).joined(separator: " · ")
    }

    private var undoSubtitle: String {
        if let burst = app.state.burst {
            let n = burst.fileCount
            let files = "\(n) file\(n == 1 ? "" : "s")"
            if let root = burst.root, !root.isEmpty {
                return "\(files) in \(URL(fileURLWithPath: root).lastPathComponent)"
            }
            return "\(files) in the last burst"
        }
        return "Nothing to restore"
    }
}

private struct MenuRow: View {
    var title: String
    var subtitle: String?
    var symbol: String
    var disabled: Bool = false
    var action: () -> Void

    @State private var hover = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: symbol)
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(LeashPaint.muted)
                    .frame(width: 16)
                VStack(alignment: .leading, spacing: 1) {
                    Text(title)
                        .font(.system(size: 13, weight: .medium))
                        .foregroundStyle(LeashPaint.ink)
                    if let subtitle, !subtitle.isEmpty {
                        Text(subtitle)
                            .font(.system(size: 11))
                            .foregroundStyle(LeashPaint.muted)
                            .lineLimit(1)
                            .truncationMode(.middle)
                    }
                }
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, subtitle == nil ? 7 : 8)
            .background(hover && !disabled ? LeashPaint.faint : .clear, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            .contentShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        }
        .buttonStyle(.plain)
        .disabled(disabled)
        .opacity(disabled ? 0.42 : 1)
        .onHover { hover = $0 }
    }
}
