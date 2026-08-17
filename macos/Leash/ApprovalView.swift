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
                empty
            }
        }
        .frame(width: 432)
        .leashWindowFill()
        .background(WindowAccess(configure: LeashChrome.approval))
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

    private var empty: some View {
        VStack(spacing: 10) {
            LeashMark(filled: true, tint: LeashPaint.muted, size: 14)
            Text("Caught up")
                .font(.system(size: 15, weight: .medium))
                .foregroundStyle(LeashPaint.ink)
            Text("No tool call waiting.")
                .font(.system(size: 12))
                .foregroundStyle(LeashPaint.muted)
        }
        .frame(maxWidth: .infinity, minHeight: 168)
        .padding(28)
    }

    private func panel(_ pending: PendingApproval) -> some View {
        let kind = LeashKind(pending.kind)
        return VStack(alignment: .leading, spacing: 0) {
            header(pending, kind: kind)
            titleBlock(pending)
                .padding(.top, 18)
            commandWell(pending, kind: kind)
                .padding(.top, 16)
            if let cwd = pending.cwd, !cwd.isEmpty {
                pathRow(cwd)
                    .padding(.top, 10)
            }
            actions
                .padding(.top, 22)
        }
        .padding(.horizontal, 22)
        .padding(.top, 18)
        .padding(.bottom, 18)
    }

    private func header(_ pending: PendingApproval, kind: LeashKind) -> some View {
        HStack(spacing: 8) {
            LeashMark(filled: true, tint: kind.color, size: 14)
            LeashWordmark()
            Spacer()
            if !pending.tool.isEmpty {
                Text(pending.tool)
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundStyle(LeashPaint.muted)
            }
            KindChip(kind: kind)
        }
    }

    private func titleBlock(_ pending: PendingApproval) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(pending.title)
                .font(.system(size: 22, weight: .semibold))
                .tracking(-0.4)
                .foregroundStyle(LeashPaint.ink)
            if !pending.reasons.isEmpty {
                Text(pending.reasons.joined(separator: "  ·  "))
                    .font(.system(size: 13))
                    .foregroundStyle(LeashPaint.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func commandWell(_ pending: PendingApproval, kind: LeashKind) -> some View {
        let body = Text(highlightedCommand(pending.detail, needles: pending.reasons, accent: kind.color))
            .textSelection(.enabled)
            .lineSpacing(3)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 14)
            .padding(.vertical, 12)

        return HStack(alignment: .top, spacing: 0) {
            kind.color
                .frame(width: 2)
            ViewThatFits(in: .vertical) {
                body
                ScrollView(.vertical, showsIndicators: false) {
                    body
                }
                .frame(maxHeight: 160)
            }
        }
        .background(LeashPaint.well)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 10, style: .continuous)
                .strokeBorder(LeashPaint.hairline, lineWidth: 1)
        )
    }

    private func pathRow(_ cwd: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: "folder")
                .font(.system(size: 10, weight: .medium))
            Text(compactPath(cwd))
                .font(.system(size: 11, design: .monospaced))
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .foregroundStyle(LeashPaint.muted)
    }

    private var actions: some View {
        HStack(spacing: 8) {
            actionButton("Kill", hint: "esc", kind: .kill) {
                Task { await app.decide("kill") }
            }
            .keyboardShortcut(.cancelAction)
            .disabled(app.deciding)

            Spacer(minLength: 8)

            actionButton("Always", hint: "⌘↩", kind: .always) {
                Task { await app.decide("always") }
            }
            .keyboardShortcut(.return, modifiers: [.command])
            .disabled(app.deciding)

            actionButton("Allow", hint: "↩", kind: .allow) {
                Task { await app.decide("allow") }
            }
            .keyboardShortcut(.defaultAction)
            .disabled(app.deciding)
        }
        .opacity(app.deciding ? 0.55 : 1)
        .animation(.easeOut(duration: 0.15), value: app.deciding)
    }

    private enum ActionKind { case kill, always, allow }

    private func actionButton(_ title: String, hint: String, kind: ActionKind, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 8) {
                Text(title)
                    .font(.system(size: 13, weight: kind == .always ? .medium : .semibold))
                KeyHint(keys: hint, on: hintTone(kind))
            }
            .padding(.horizontal, kind == .always ? 11 : 14)
            .frame(height: 34)
            .background(actionFill(kind), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 8, style: .continuous)
                    .strokeBorder(kind == .always ? LeashPaint.hairline : .clear, lineWidth: 1)
            )
            .foregroundStyle(actionInk(kind))
        }
        .buttonStyle(.plain)
    }

    private func actionFill(_ kind: ActionKind) -> Color {
        switch kind {
        case .kill: return LeashPaint.vermillion
        case .always: return LeashPaint.faint
        case .allow: return LeashPaint.ink
        }
    }

    private func actionInk(_ kind: ActionKind) -> Color {
        switch kind {
        case .kill: return LeashPaint.bone
        case .always: return LeashPaint.ink
        case .allow: return LeashPaint.paper
        }
    }

    private func hintTone(_ kind: ActionKind) -> KeyHint.Tone {
        switch kind {
        case .kill: return .vermillion
        case .always: return .paper
        case .allow: return .ink
        }
    }

    private func polishWindow() {
        for window in NSApp.windows where window.identifier?.rawValue == "approval" || window.title == "Leash" {
            LeashChrome.approval(window)
        }
    }
}
