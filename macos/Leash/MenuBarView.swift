import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            LeashStatusHeader(
                title: LeashFormat.statusTitle(waiting: app.state.waitingCount, offline: app.daemonError != nil, status: app.state.status, connecting: app.connecting),
                detail: LeashFormat.statusDetail(pending: app.state.pending, error: app.daemonError, folders: app.state.folders, connecting: app.connecting),
                tint: LeashFormat.statusTint(waiting: app.state.waitingCount, offline: app.daemonError != nil, status: app.state.status, connecting: app.connecting),
                filled: LeashFormat.markFilled(waiting: app.state.waitingCount, status: app.state.status),
                activity: LeashFormat.statusActivity(
                    connecting: app.connecting,
                    waiting: app.state.waitingCount,
                    working: LeashFormat.missionLive(phase: app.state.mission?.phase, pending: app.state.pending != nil, jobActive: app.state.job?.isActive == true) && app.state.waitingCount == 0
                )
            )
            Hairline()
                .padding(.top, LeashSpace.xl)
                .padding(.bottom, LeashSpace.sm)

            if let line = LeashFormat.agentLine(app.state.agents) {
                Text(line)
                    .font(LeashType.caption)
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(2)
                    .padding(.horizontal, LeashSpace.md)
                    .padding(.bottom, LeashSpace.sm)
            }

            if app.connecting && app.state.allPending.isEmpty {
                HStack(spacing: LeashSpace.lg) {
                    LeashLoader(mode: .fasten, size: .inline, tint: LeashPaint.moss)
                    VStack(alignment: .leading, spacing: LeashSpace.xxs) {
                        Text(LeashCopy.starting)
                            .font(LeashType.rowStrong)
                            .foregroundStyle(LeashPaint.ink)
                        Text(LeashCopy.startingDetail)
                            .font(LeashType.caption)
                            .foregroundStyle(LeashPaint.muted)
                    }
                    Spacer(minLength: 0)
                }
                .padding(.horizontal, LeashSpace.md)
                .padding(.vertical, LeashSpace.md)
            }

            ForEach(Array(app.state.allPending.enumerated()), id: \.element.id) { i, pending in
                LeashPendingRow(pending: pending, queued: i > 0) {
                    ApprovalHUD.shared.show(model: app)
                }
                .padding(.bottom, LeashSpace.sm)
            }
            if !app.state.allPending.isEmpty {
                Hairline()
                    .padding(.bottom, LeashSpace.sm)
            }

            LeashKicker(text: LeashCopy.job)
                .padding(.horizontal, LeashSpace.md)
                .padding(.top, LeashSpace.xs)
                .padding(.bottom, LeashSpace.xxs)

            LeashMenuRow(title: LeashCopy.mission, subtitle: LeashFormat.missionSubtitle(phase: app.state.phase, title: app.state.mission?.title), symbol: LeashSymbol.mission) {
                app.openMission()
            }
            LeashMenuRow(
                title: LeashCopy.sendJob,
                subtitle: sendSubtitle,
                symbol: LeashSymbol.send,
                disabled: app.state.job?.isActive == true
            ) {
                app.openMission()
            }

            folderRows
            alwaysRows

            LeashMenuRow(
                title: LeashCopy.undoLastBurst,
                subtitle: LeashFormat.undoSubtitle(app.state.burst),
                symbol: LeashSymbol.undo,
                disabled: app.state.burst == nil
            ) {
                Task { await app.undo() }
            }
            LeashMenuRow(title: LeashCopy.installHooks, subtitle: LeashFormat.installSubtitle(app.state.agents), symbol: LeashSymbol.install) {
                Task { await app.installHooks() }
            }

            if let notice = app.notice {
                LeashNotice(text: notice)
            }

            Hairline()
                .padding(.vertical, LeashSpace.sm)

            LeashMenuRow(title: LeashCopy.quitLeash, symbol: LeashSymbol.quit) {
                NSApplication.shared.terminate(nil)
            }
        }
        .padding(LeashSpace.lg)
        .frame(width: LeashLayout.menuWidth)
        .leashChrome(LeashChrome.menu)
        .onAppear { app.start() }
    }

    private var sendSubtitle: String {
        if app.state.job?.isActive == true {
            return app.state.job?.agent ?? LeashCopy.working
        }
        let found = LeashFormat.spawnable(app.state.agents)
        if found.isEmpty { return LeashCopy.noSpawn }
        return found.map(\.name).joined(separator: LeashCopy.dot)
    }

    @ViewBuilder
    private var folderRows: some View {
        let folders = app.state.folders
        if folders.isEmpty {
            LeashMenuRow(title: LeashCopy.watchFolders, subtitle: LeashCopy.chooseFolders, symbol: LeashSymbol.folder) {
                app.pickFolder()
            }
        } else {
            LeashKicker(text: LeashCopy.foldersKicker)
                .padding(.horizontal, LeashSpace.md)
                .padding(.top, LeashSpace.sm)
                .padding(.bottom, LeashSpace.xxs)
            ForEach(folders.prefix(LeashLayout.folderCap), id: \.self) { path in
                LeashRemovableRow(
                    title: LeashFormat.folderName(path),
                    subtitle: LeashFormat.compactPath(path),
                    symbol: LeashSymbol.folder
                ) {
                    Task { await app.unwatch(path) }
                }
            }
            LeashMenuRow(title: LeashCopy.addFolder, symbol: LeashSymbol.addFolder) {
                app.pickFolder()
            }
        }
    }

    @ViewBuilder
    private var alwaysRows: some View {
        let rules = app.state.alwaysAllow
        if !rules.isEmpty {
            LeashKicker(text: LeashCopy.alwaysRules)
                .padding(.horizontal, LeashSpace.md)
                .padding(.top, LeashSpace.sm)
                .padding(.bottom, LeashSpace.xxs)
            ForEach(rules.prefix(LeashLayout.alwaysCap)) { rule in
                LeashRemovableRow(
                    title: rule.pattern,
                    subtitle: LeashFormat.alwaysSubtitle(rule),
                    symbol: LeashSymbol.alwaysList
                ) {
                    Task { await app.revokeAlways(rule) }
                }
            }
        }
    }
}
