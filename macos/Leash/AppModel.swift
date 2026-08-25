import AppKit
import Combine
import Foundation

@MainActor
final class AppModel: ObservableObject {
    static let shared = AppModel()

    @Published var state: LeashState = .empty
    @Published var daemonError: String?
    @Published var lastUndo: String?
    @Published var notice: String?
    @Published var deciding = false
    @Published var connecting = true
    @Published var selectedEventID: String?
    @Published var steerDraft = ""
    @Published var jobDraft = ""
    @Published var jobAgentID = ""
    @Published var sendingJob = false

    private var client = DaemonClient()
    private var timer: Timer?
    private var lastPendingID: String?
    private var process: Process?

    private var started = false

    func start() {
        guard !started else { return }
        started = true
        Task { await bootstrap() }
        timer = Timer.scheduledTimer(withTimeInterval: LeashMotion.poll, repeats: true) { [weak self] _ in
            Task { @MainActor in
                await self?.refresh()
            }
        }
    }

    func stop() {
        timer?.invalidate()
        timer = nil
        process?.terminate()
        process = nil
    }

    func refresh() async {
        do {
            let next = try await client.state()
            let prevLast = state.mission?.timeline.last?.id
            let appeared = next.pending != nil && next.pending?.id != lastPendingID
            let missionLive = LeashFormat.missionLive(phase: next.mission?.phase, pending: next.pending != nil, jobActive: next.job?.isActive == true)
            state = next
            daemonError = nil
            connecting = false
            if selectedEventID == nil || selectedEventID == prevLast, let last = next.mission?.timeline.last?.id {
                selectedEventID = last
            }
            if appeared {
                lastPendingID = next.pending?.id
                presentApproval()
            }
            if missionLive {
                MissionHUD.shared.show(model: self)
            }
            if next.pending == nil {
                lastPendingID = nil
                ApprovalHUD.shared.hide()
            }
        } catch {
            if connecting { return }
            state = .empty
            daemonError = error.localizedDescription
        }
    }

    func decide(_ action: String) async {
        guard let id = state.pending?.id else { return }
        deciding = true
        NSHapticFeedbackManager.defaultPerformer.perform(.generic, performanceTime: .now)
        defer { deciding = false }
        do {
            try await client.decide(id: id, action: action)
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func undo() async {
        do {
            let n = try await client.undo()
            lastUndo = LeashCopy.restored(n)
            notice = lastUndo
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func steer() async {
        let text = steerDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        do {
            try await client.steer(text)
            steerDraft = ""
            notice = LeashCopy.steering
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func interrupt() async {
        let text = steerDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        NSHapticFeedbackManager.defaultPerformer.perform(.generic, performanceTime: .now)
        do {
            if state.pending != nil {
                await decide("kill")
            } else {
                try await client.interrupt(text)
                steerDraft = ""
                await refresh()
            }
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func retry() async {
        do {
            try await client.retry()
            notice = LeashCopy.retryArmed
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func skipFail() async {
        do {
            try await client.skip()
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func sendJob() async {
        let text = jobDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        if state.folders.isEmpty {
            pickFolder()
            return
        }
        sendingJob = true
        defer { sendingJob = false }
        do {
            try await client.run(
                task: text,
                agent: jobAgentID,
                cwd: state.folders.first ?? "",
                fallback: jobAgentID.isEmpty
            )
            jobDraft = ""
            notice = LeashCopy.jobSent
            openMission()
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func openMission() {
        MissionHUD.shared.show(model: self)
    }

    func pickFolder() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.message = LeashCopy.addFolderPrompt
        guard panel.runModal() == .OK, let url = panel.url else { return }
        Task {
            do {
                try await client.watch(url.path)
                await refresh()
            } catch {
                daemonError = error.localizedDescription
            }
        }
    }

    func unwatch(_ path: String) async {
        do {
            try await client.unwatch(path)
            notice = LeashCopy.unwatched
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func revokeAlways(_ rule: AlwaysRule) async {
        do {
            try await client.revokeAlways(rule)
            notice = LeashCopy.revoked
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func installHooks() async {
        guard let bin = bundledLeash() else {
            daemonError = LeashCopy.helperMissing
            return
        }
        let proc = Process()
        proc.executableURL = bin
        proc.arguments = ["install"]
        do {
            try proc.run()
            proc.waitUntilExit()
            if proc.terminationStatus != 0 {
                daemonError = LeashCopy.installFailed
            } else {
                notice = LeashCopy.hooksInstalled
            }
        } catch {
            daemonError = error.localizedDescription
        }
    }

    private func bootstrap() async {
        defer { connecting = false }
        if await client.reachable() {
            await refresh()
            return
        }
        launchHelper()
        for _ in 0 ..< LeashMotion.bootstrapTries {
            try? await Task.sleep(nanoseconds: LeashMotion.bootstrapTickNs)
            if await client.reachable() {
                await refresh()
                return
            }
        }
        daemonError = LeashCopy.couldNotStart
    }

    private func launchHelper() {
        guard let bin = bundledLeash() else { return }
        let proc = Process()
        proc.executableURL = bin
        proc.arguments = ["serve"]
        proc.standardOutput = FileHandle.nullDevice
        proc.standardError = FileHandle.nullDevice
        do {
            try proc.run()
            process = proc
        } catch {
            daemonError = error.localizedDescription
        }
    }

    private func bundledLeash() -> URL? {
        if let url = Bundle.main.url(forResource: "leash", withExtension: nil) {
            return url
        }
        let home = FileManager.default.homeDirectoryForCurrentUser
        let fallback = home.appendingPathComponent(".leash/bin/leash")
        if FileManager.default.isExecutableFile(atPath: fallback.path) {
            return fallback
        }
        return nil
    }

    private func presentApproval() {
        ApprovalHUD.shared.show(model: self)
    }
}
