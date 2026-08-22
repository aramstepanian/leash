import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        let agent = LeashFormat.dispatchAgent(app.state.agents, prefer: app.selectedAgentID)
        let job = app.state.job
        VStack(alignment: .leading, spacing: 0) {
            LeashStatusHeader(
                title: LeashFormat.dispatchTitle(offline: app.daemonError != nil, folder: app.workFolder, agent: agent, job: job),
                detail: LeashFormat.dispatchDetail(offlineError: app.daemonError, folder: app.workFolder, agent: agent, job: job),
                tint: LeashFormat.dispatchTint(offline: app.daemonError != nil, folder: app.workFolder, agent: agent, job: job),
                filled: LeashFormat.dispatchFilled(job: job)
            )
            Hairline()
                .padding(.top, LeashSpace.xl)
                .padding(.bottom, LeashSpace.sm)

            LeashField(
                placeholder: LeashCopy.promptPlaceholder,
                text: $app.promptDraft,
                onSubmit: { Task { await app.send() } }
            )
            .disabled(job?.running == true || app.sending || app.daemonError != nil)
            .padding(.horizontal, LeashSpace.md)
            .padding(.bottom, LeashSpace.sm)

            agentPicker
                .padding(.horizontal, LeashSpace.md)
                .padding(.bottom, LeashSpace.sm)

            if let job, job.status == "done" || job.status == "failed" {
                let text = LeashFormat.plainText(job.displayText)
                if !text.isEmpty {
                    Text(text)
                        .font(LeashType.caption)
                        .foregroundStyle(job.status == "failed" ? LeashPaint.vermillion : LeashPaint.muted)
                        .textSelection(.enabled)
                        .lineLimit(8)
                        .padding(.horizontal, LeashSpace.md)
                        .padding(.bottom, LeashSpace.sm)
                }
            }

            folderRow

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

    private var agentPicker: some View {
        let choices = LeashFormat.dispatchChoices(app.state.agents)
        let selected = LeashFormat.dispatchAgent(app.state.agents, prefer: app.selectedAgentID)?.id
        let locked = app.state.job?.running == true || app.sending
        return LazyVGrid(
            columns: [GridItem(.adaptive(minimum: 72), spacing: LeashSpace.sm, alignment: .leading)],
            alignment: .leading,
            spacing: LeashSpace.sm
        ) {
            ForEach(choices) { choice in
                Button {
                    app.selectAgent(choice.id)
                } label: {
                    LeashChip(
                        title: LeashFormat.pickerName(choice),
                        tint: choice.installed ? LeashPaint.steel : LeashPaint.muted,
                        on: choice.id == selected
                    )
                }
                .buttonStyle(.plain)
                .disabled(!choice.installed || locked)
                .opacity(choice.installed ? 1 : LeashPaint.Opacity.disabled)
                .help(choice.installed ? LeashFormat.pickerName(choice) : LeashCopy.notInstalled)
            }
        }
        .accessibilityLabel(LeashCopy.pickAgent)
    }

    private var folderRow: some View {
        let path = app.workFolder
        return LeashMenuRow(
            title: path.map(LeashFormat.folderName) ?? LeashCopy.pickWorkFolder,
            subtitle: path.map(LeashFormat.compactPath) ?? LeashCopy.chooseWorkFolder,
            symbol: LeashSymbol.folder
        ) {
            app.pickFolder()
        }
    }
}
