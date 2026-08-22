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
                filled: LeashFormat.dispatchFilled(job: job),
                running: job?.running == true
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

            if job?.running == true || app.sending {
                let live = LeashFormat.replyBody(job?.result ?? "")
                if live.isEmpty {
                    LeashRunningPulse()
                        .padding(.horizontal, LeashSpace.md)
                        .padding(.bottom, LeashSpace.sm)
                } else {
                    VStack(alignment: .leading, spacing: LeashSpace.xs) {
                        LeashKicker(text: LeashCopy.working)
                        LeashReplyWell(text: live)
                    }
                    .padding(.horizontal, LeashSpace.md)
                    .padding(.bottom, LeashSpace.sm)
                }
            } else if let job, job.status == "done" || job.status == "failed" {
                let text = LeashFormat.replyBody(job.displayText)
                if !text.isEmpty {
                    LeashReplyWell(text: text, failed: job.status == "failed")
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
        return VStack(alignment: .leading, spacing: LeashSpace.xs) {
            LeashKicker(text: LeashCopy.pickAgent)
            LazyVGrid(
                columns: [GridItem(.adaptive(minimum: 72), spacing: LeashSpace.sm, alignment: .leading)],
                alignment: .leading,
                spacing: LeashSpace.sm
            ) {
                ForEach(choices) { choice in
                    Button {
                        pick(choice, locked: locked)
                    } label: {
                        LeashChip(
                            title: LeashFormat.pickerName(choice),
                            tint: choice.installed ? LeashPaint.steel : LeashPaint.muted,
                            on: choice.id == selected
                        )
                    }
                    .buttonStyle(.plain)
                    .disabled(locked)
                    .opacity(choice.installed ? 1 : LeashPaint.Opacity.disabled)
                    .help(choiceHelp(choice))
                }
            }
        }
        .accessibilityLabel(LeashCopy.pickAgent)
    }

    private func pick(_ choice: AgentInfo, locked: Bool) {
        guard !locked else { return }
        if choice.installed {
            app.selectAgent(choice.id)
            app.notice = nil
            return
        }
        app.notice = "\(LeashFormat.pickerName(choice)) · \(LeashCopy.notInstalled)"
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

    private func choiceHelp(_ choice: AgentInfo) -> String {
        if !choice.installed { return LeashCopy.notInstalled }
        if choice.id == "cursor-cli", let path = choice.path, path.hasSuffix(".app") {
            return LeashCopy.cursorNeedsCLI
        }
        return LeashFormat.pickerName(choice)
    }
}
