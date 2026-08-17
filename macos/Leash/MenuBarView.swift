import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            statusRow
            Divider()
            if let pending = app.state.pending {
                Button("Review \(pending.title)…") {
                    openWindow(id: "approval")
                }
                Divider()
            }
            Button("Watch folder…") { app.pickFolder() }
            if let root = app.state.watchRoot, !root.isEmpty {
                Text(compact(root))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 12)
            }
            Button("Undo last burst") {
                Task { await app.undo() }
            }
            .disabled(app.state.burst == nil)
            if let burst = app.state.burst {
                Text("\(burst.fileCount) files in last burst")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 12)
            }
            if let msg = app.lastUndo {
                Text(msg)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 12)
            }
            Divider()
            Button("Install agent hooks") {
                Task { await app.installHooks() }
            }
            Text("Cursor, OpenCode, Claude, Codex")
                .font(.caption)
                .foregroundStyle(.secondary)
                .padding(.horizontal, 12)
            Button("Quit Leash") {
                NSApplication.shared.terminate(nil)
            }
        }
        .onAppear { app.start() }
        .onReceive(NotificationCenter.default.publisher(for: .leashShowApproval)) { _ in
            openWindow(id: "approval")
        }
    }

    private var statusRow: some View {
        HStack {
            Circle()
                .fill(dot)
                .frame(width: 8, height: 8)
            Text(label)
            Spacer()
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 6)
    }

    private var label: String {
        if app.state.pending != nil { return "Waiting on you" }
        if app.daemonError != nil { return "Daemon off" }
        switch app.state.status {
        case "watching": return "Watching"
        case "waiting": return "Waiting on you"
        case "idle": return "Idle"
        default: return "Offline"
        }
    }

    private var dot: Color {
        if app.state.pending != nil { return .red }
        if app.daemonError != nil { return .secondary }
        if app.state.status == "watching" { return .green }
        return .secondary
    }

    private func compact(_ path: String) -> String {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        if path.hasPrefix(home) {
            return "~" + path.dropFirst(home.count)
        }
        return path
    }
}
