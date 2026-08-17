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

    private var client = DaemonClient()
    private var timer: Timer?
    private var lastPendingID: String?
    private var process: Process?

    private var started = false

    func start() {
        guard !started else { return }
        started = true
        Task { await bootstrap() }
        timer = Timer.scheduledTimer(withTimeInterval: 0.35, repeats: true) { [weak self] _ in
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
            let appeared = next.pending != nil && next.pending?.id != lastPendingID
            state = next
            daemonError = nil
            if appeared {
                lastPendingID = next.pending?.id
                presentApproval()
            }
            if next.pending == nil {
                lastPendingID = nil
                ApprovalHUD.shared.hide()
            }
        } catch {
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
            lastUndo = "Restored \(n) file\(n == 1 ? "" : "s")"
            notice = lastUndo
            await refresh()
        } catch {
            daemonError = error.localizedDescription
        }
    }

    func pickFolder() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = false
        panel.canChooseDirectories = true
        panel.allowsMultipleSelection = false
        panel.message = "Add a folder Leash should protect"
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

    func installHooks() async {
        guard let bin = bundledLeash() else {
            daemonError = "leash helper not found — run make install"
            return
        }
        let proc = Process()
        proc.executableURL = bin
        proc.arguments = ["install"]
        do {
            try proc.run()
            proc.waitUntilExit()
            if proc.terminationStatus != 0 {
                daemonError = "install failed"
            } else {
                notice = "Hooks installed"
            }
        } catch {
            daemonError = error.localizedDescription
        }
    }

    private func bootstrap() async {
        if await client.reachable() {
            await refresh()
            return
        }
        launchHelper()
        for _ in 0 ..< 20 {
            try? await Task.sleep(nanoseconds: 150_000_000)
            if await client.reachable() {
                await refresh()
                return
            }
        }
        daemonError = "Could not start leash serve"
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
