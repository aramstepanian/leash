import AppKit
import SwiftUI

struct MenuBarView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        let agent = LeashFormat.dispatchAgent(app.state.agents, prefer: app.selectedAgentID)
        let job = app.state.job
        let busy = job?.running == true || app.sending
        VStack(alignment: .leading, spacing: LeashSpace.section) {
            LeashStatusHeader(
                title: LeashFormat.dispatchTitle(
                    offline: app.daemonError != nil,
                    folder: app.workFolder,
                    agent: agent,
                    job: job,
                    connecting: app.connecting
                ),
                detail: LeashFormat.dispatchDetail(
                    offlineError: app.daemonError,
                    folder: app.workFolder,
                    agent: agent,
                    job: job,
                    connecting: app.connecting
                ),
                tint: LeashFormat.dispatchTint(offline: app.daemonError != nil, folder: app.workFolder, agent: agent, job: job),
                filled: LeashFormat.dispatchFilled(job: job),
                activity: {
                    if app.connecting { return .connecting }
                    if busy { return .working }
                    return .rest
                }()
            )

            section(LeashCopy.promptKicker) {
                LeashField(
                    placeholder: LeashCopy.promptPlaceholder,
                    text: $app.promptDraft,
                    onSubmit: { Task { await app.send() } }
                )
                .disabled(busy || app.daemonError != nil)
            }

            if let reply = replyText(job: job, busy: busy) {
                section(LeashCopy.replyKicker) {
                    LeashReplyWell(text: reply.text, failed: reply.failed)
                }
            }

            VStack(alignment: .leading, spacing: LeashSpace.lg) {
                agentPicker
                folderBlock
            }

            if let notice = app.notice {
                LeashNotice(text: notice)
            }

            VStack(alignment: .leading, spacing: LeashSpace.sm) {
                Hairline()
                LeashMenuRow(title: LeashCopy.quitLeash, symbol: LeashSymbol.quit) {
                    NSApplication.shared.terminate(nil)
                }
            }
        }
        .padding(LeashSpace.lg)
        .frame(width: LeashLayout.menuWidth)
        .leashChrome(LeashChrome.menu)
        .onAppear { app.start() }
    }

    private func section<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: LeashSpace.xs) {
            LeashKicker(text: title)
            content()
        }
    }

    private func replyText(job: JobInfo?, busy: Bool) -> (text: String, failed: Bool)? {
        guard !app.connecting else { return nil }
        if busy {
            let live = LeashFormat.replyBody(job?.result ?? "")
            return live.isEmpty ? nil : (live, false)
        }
        guard let job, job.status == "done" || job.status == "failed" else { return nil }
        let text = LeashFormat.replyBody(job.displayText)
        return text.isEmpty ? nil : (text, job.status == "failed")
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

    private var folderBlock: some View {
        let path = app.workFolder
        return VStack(alignment: .leading, spacing: LeashSpace.xs) {
            LeashKicker(text: LeashCopy.folderKicker)
            LeashMenuRow(
                title: path.map(LeashFormat.folderName) ?? LeashCopy.pickWorkFolder,
                subtitle: path.map(LeashFormat.compactPath) ?? LeashCopy.chooseWorkFolder,
                symbol: LeashSymbol.folder
            ) {
                app.pickFolder()
            }
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
