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

            if !app.state.allPending.isEmpty {
                ForEach(app.state.allPending) { pending in
                    pendingRow(pending)
                        .padding(.bottom, 6)
                }
                Hairline()
                    .padding(.bottom, 6)
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
                LeashMark(filled: app.state.pending != nil || app.state.status == "watching", tint: statusTint, size: 14)
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

    private func pendingRow(_ pending: PendingApproval) -> some View {
        let kind = LeashKind(pending.kind)
        return Button {
            ApprovalHUD.shared.show(model: app)
        } label: {
            HStack(spacing: 10) {
                RoundedRectangle(cornerRadius: 2, style: .continuous)
                    .fill(kind.color)
                    .frame(width: 2, height: 28)
                VStack(alignment: .leading, spacing: 2) {
                    Text(pending.title)
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(LeashPaint.ink)
                    Text(pendingWaitLabel(pending))
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
            .background(kind.color.opacity(0.10), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
        }
        .buttonStyle(.plain)
    }

    private func pendingWaitLabel(_ pending: PendingApproval) -> String {
        var parts: [String] = []
        if let agent = pending.agent, !agent.isEmpty { parts.append(agent) }
        let folder = pending.root ?? pending.cwd ?? ""
        if !folder.isEmpty {
            parts.append((folder as NSString).lastPathComponent)
        }
        if parts.isEmpty { return "Waiting on you" }
        return parts.joined(separator: " · ")
    }

    private var statusTitle: String {
        let n = app.state.waitingCount
        if n > 1 { return "Needs you · \(n)" }
        if n == 1 { return "Needs you" }
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
            return pending.title
        }
        if let err = app.daemonError {
            return err
        }
        let folders = app.state.folders
        if folders.count > 1 {
            return "\(folders.count) folders"
        }
        if let root = folders.first {
            return compactPath(root)
        }
        return "Pick a folder to protect"
    }

    private var statusTint: Color {
        if app.state.pending != nil { return LeashPaint.vermillion }
        if app.daemonError != nil { return LeashPaint.muted }
        if app.state.status == "watching" { return Color(nsColor: NSColor(srgbRed: 0.310, green: 0.545, blue: 0.427, alpha: 1)) }
        return LeashPaint.muted
    }

    private var watchSubtitle: String {
        let folders = app.state.folders
        if folders.count > 1 {
            return folders.map { ($0 as NSString).lastPathComponent }.joined(separator: " · ")
        }
        if let root = folders.first {
            return compactPath(root)
        }
        return "Choose the project folder"
    }

    private var undoSubtitle: String {
        if let burst = app.state.burst {
            let n = burst.fileCount
            var line = "\(n) file\(n == 1 ? "" : "s")"
            if let root = burst.root, !root.isEmpty {
                line += " in \((root as NSString).lastPathComponent)"
            } else {
                line += " in the last burst"
            }
            return line
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
