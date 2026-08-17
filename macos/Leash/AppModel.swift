import AppKit
import Combine
import Foundation

@MainActor
final class AppModel: ObservableObject {
    @Published var state: LeashState = .empty
    @Published var daemonError: String?
    @Published var lastUndo: String?

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
        process?.terminate()
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
            }
        } catch {
            state = .empty
            daemonError = error.localizedDescription
        }
    }

    func decide(_ action: String) async {
        guard let id = state.pending?.id else { return }
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
        panel.message = "Folder Leash should protect"
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
            }
        } catch {
            daemonError = error.localizedDescription
        }
    }

    private func bootstrap() async {
        if client.reachable() {
            await refresh()
            return
        }
        launchHelper()
        for _ in 0 ..< 20 {
            try? await Task.sleep(nanoseconds: 150_000_000)
            if client.reachable() {
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
        NSApp.activate(ignoringOtherApps: true)
        if let window = NSApp.windows.first(where: { $0.identifier?.rawValue == "approval" }) {
            window.makeKeyAndOrderFront(nil)
            window.level = .floating
            return
        }
        openApprovalWindow()
    }

    private func openApprovalWindow() {
        for scene in NSApp.windows where scene.title == "Leash" {
            scene.makeKeyAndOrderFront(nil)
            scene.level = .floating
        }
        NSApp.activate(ignoringOtherApps: true)
        if let env = ProcessInfo.processInfo.environment["APP_SANDBOX_CONTAINER_ID"] {
            _ = env
        }
        // SwiftUI Window is opened via openWindow in the view; post a note.
        NotificationCenter.default.post(name: .leashShowApproval, object: nil)
    }
}

extension Notification.Name {
    static let leashShowApproval = Notification.Name("leashShowApproval")
}
