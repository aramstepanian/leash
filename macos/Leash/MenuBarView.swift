import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            LeashStatusHeader(
                title: LeashFormat.statusTitle(waiting: app.state.waitingCount, offline: app.daemonError != nil, status: app.state.status),
                detail: LeashFormat.statusDetail(pending: app.state.pending, error: app.daemonError, folders: app.state.folders),
                tint: LeashFormat.statusTint(waiting: app.state.waitingCount, offline: app.daemonError != nil, status: app.state.status),
                filled: LeashFormat.markFilled(waiting: app.state.waitingCount, status: app.state.status)
            )
            Hairline()
                .padding(.top, LeashSpace.xl)
                .padding(.bottom, LeashSpace.sm)

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

            LeashMenuRow(title: LeashCopy.mission, subtitle: LeashFormat.missionSubtitle(phase: app.state.phase, title: app.state.mission?.title), symbol: LeashSymbol.mission) {
                app.openMission()
            }
            LeashMenuRow(title: LeashCopy.watchFolders, subtitle: LeashFormat.watchSubtitle(app.state.folders), symbol: LeashSymbol.folder) {
                app.pickFolder()
            }
            LeashMenuRow(
                title: LeashCopy.undoLastBurst,
                subtitle: LeashFormat.undoSubtitle(app.state.burst),
                symbol: LeashSymbol.undo,
                disabled: app.state.burst == nil
            ) {
                Task { await app.undo() }
            }
            LeashMenuRow(title: LeashCopy.installHooks, subtitle: LeashCopy.installAgents, symbol: LeashSymbol.install) {
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
}
