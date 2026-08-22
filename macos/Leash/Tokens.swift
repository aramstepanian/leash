import AppKit
import SwiftUI

enum LeashPaint {
    static let paperNS = NSColor(name: nil, dynamicProvider: { appearance in
        appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
            ? NSColor(srgbRed: 0.110, green: 0.106, blue: 0.098, alpha: 1)
            : NSColor(srgbRed: 0.957, green: 0.945, blue: 0.925, alpha: 1)
    })
    static let paper = Color(nsColor: paperNS)
    static let ink = Color(
        light: NSColor(srgbRed: 0.102, green: 0.098, blue: 0.086, alpha: 1),
        dark: NSColor(srgbRed: 0.949, green: 0.937, blue: 0.910, alpha: 1)
    )
    static let muted = ink.opacity(Opacity.muted)
    static let faint = ink.opacity(Opacity.faint)
    static let hairline = ink.opacity(Opacity.hairline)
    static let well = Color(
        light: NSColor(srgbRed: 0.922, green: 0.906, blue: 0.878, alpha: 1),
        dark: NSColor(srgbRed: 0.078, green: 0.075, blue: 0.070, alpha: 1)
    )
    static let vermillion = Color(nsColor: NSColor(srgbRed: 0.839, green: 0.271, blue: 0.196, alpha: 1))
    static let bone = Color(nsColor: NSColor(srgbRed: 0.969, green: 0.953, blue: 0.933, alpha: 1))
    static let amber = Color(nsColor: NSColor(srgbRed: 0.788, green: 0.518, blue: 0.165, alpha: 1))
    static let moss = Color(nsColor: NSColor(srgbRed: 0.310, green: 0.545, blue: 0.427, alpha: 1))
    static let steel = Color(nsColor: NSColor(srgbRed: 0.353, green: 0.478, blue: 0.659, alpha: 1))
    static let code = ink.opacity(Opacity.code)

    enum Opacity {
        static let muted: CGFloat = 0.52
        static let faint: CGFloat = 0.08
        static let hairline: CGFloat = 0.14
        static let code: CGFloat = 0.90
        static let chipOn: CGFloat = 0.14
        static let chipOff: CGFloat = 0.05
        static let pending: CGFloat = 0.10
        static let queued: CGFloat = 0.06
        static let statusHalo: CGFloat = 0.14
        static let hintOnInk: CGFloat = 0.14
        static let hintOnKill: CGFloat = 0.18
        static let hintInk: CGFloat = 0.72
        static let hintKill: CGFloat = 0.82
        static let disabled: CGFloat = 0.42
        static let deciding: CGFloat = 0.55
        static let pulse: CGFloat = 0.35
        static let kindOther: CGFloat = 0.7
    }
}

enum LeashSpace {
    static let lead: CGFloat = 1
    static let xxs: CGFloat = 2
    static let chipY: CGFloat = 3
    static let xs: CGFloat = 4
    static let sm: CGFloat = 6
    static let hintX: CGFloat = 5
    static let rowTight: CGFloat = 7
    static let md: CGFloat = 8
    static let lg: CGFloat = 10
    static let compact: CGFloat = 11
    static let xl: CGFloat = 12
    static let xxl: CGFloat = 14
    static let section: CGFloat = 16
    static let panel: CGFloat = 18
    static let sheet: CGFloat = 22
    static let empty: CGFloat = 28

    static let hairline: CGFloat = 1
    static let rail: CGFloat = 2
    static let line: CGFloat = 3

    static let radiusChip: CGFloat = 4
    static let radiusRow: CGFloat = 6
    static let radiusControl: CGFloat = 8
    static let radiusWell: CGFloat = 10

    static let control: CGFloat = 32
    static let action: CGFloat = 34
    static let status: CGFloat = 28
    static let mark: CGFloat = 14
    static let icon: CGFloat = 16
    static let dot: CGFloat = 6
    static let inspectorFloor: CGFloat = 64
    static let commandCeiling: CGFloat = 160
    static let emptyFloor: CGFloat = 168

    enum Mark {
        static let minStroke: CGFloat = 1.4
        static let stroke: CGFloat = 0.12
        static let circle: CGFloat = 0.50
        static let strap: CGFloat = 0.30
        static let gap: CGFloat = 0.55
        static let fillPad: CGFloat = 0.95
    }
}

enum LeashLayout {
    static let menuWidth: CGFloat = 300
    static let approvalWidth: CGFloat = 432
    static let approvalFloor: CGFloat = 220
    static let approvalSeedHeight: CGFloat = 240
    static let approvalSeed = CGSize(width: approvalWidth, height: approvalSeedHeight)
    static let mission = CGSize(width: 720, height: 520)
    static let timeline: CGFloat = 280
    static let tapeFloor: CGFloat = 180
    static let approvalLift: CGFloat = 40
    static let missionInset: CGFloat = 24
    static let shotScale: CGFloat = 2
    static let previewFiles = 8
    static let alwaysCap = 6
    static let folderCap = 8
    static let highlightFloor = 2
    static let approvalID = "approval"
    static let missionID = "mission"
}

enum LeashType {
    enum Size {
        static let tape: CGFloat = 9
        static let kicker: CGFloat = 10
        static let caption: CGFloat = 11
        static let body: CGFloat = 12
        static let code: CGFloat = 12.5
        static let row: CGFloat = 13
        static let empty: CGFloat = 15
        static let display: CGFloat = 22
    }

    enum Track {
        static let display: CGFloat = -0.4
        static let chip: CGFloat = 0.4
        static let tape: CGFloat = 0.6
        static let kicker: CGFloat = 1.6
        static let wordmark: CGFloat = 1.8
    }

    static let tape = Font.system(size: Size.tape, weight: .semibold)
    static let kicker = Font.system(size: Size.kicker, weight: .semibold)
    static let chip = Font.system(size: Size.kicker, weight: .semibold)
    static let hint = Font.system(size: Size.kicker, weight: .medium, design: .monospaced)
    static let caption = Font.system(size: Size.caption)
    static let captionMedium = Font.system(size: Size.caption, weight: .medium)
    static let mono = Font.system(size: Size.caption, design: .monospaced)
    static let monoMedium = Font.system(size: Size.caption, weight: .medium, design: .monospaced)
    static let body = Font.system(size: Size.body)
    static let bodyMedium = Font.system(size: Size.body, weight: .medium)
    static let bodyStrong = Font.system(size: Size.body, weight: .semibold)
    static let code = Font.system(size: Size.body, design: .monospaced)
    static let codeMedium = Font.system(size: Size.code, weight: .medium, design: .monospaced)
    static let codeStrong = Font.system(size: Size.code, weight: .semibold, design: .monospaced)
    static let live = Font.system(size: Size.body, weight: .semibold, design: .monospaced)
    static let row = Font.system(size: Size.row, weight: .medium)
    static let rowStrong = Font.system(size: Size.row, weight: .semibold)
    static let empty = Font.system(size: Size.empty, weight: .medium)
    static let display = Font.system(size: Size.display, weight: .semibold)

    static let icon = Font.system(size: Size.kicker, weight: .medium)
    static let menuIcon = Font.system(size: Size.body, weight: .medium)
    static let chevron = Font.system(size: Size.caption, weight: .semibold)

    static func row(weight: Font.Weight) -> Font {
        .system(size: Size.row, weight: weight)
    }
}

enum LeashMotion {
    static let snap = Animation.easeOut(duration: 0.15)
    static let pulse = Animation.easeInOut(duration: 0.9).repeatForever(autoreverses: true)
    static let settle = Animation.easeOut(duration: 0.15)
    static let poll: TimeInterval = 0.35
    static let shot: TimeInterval = 0.35
    static let launchShot: TimeInterval = 0.6
    static let bootstrapTries = 20
    static let bootstrapTickNs: UInt64 = 150_000_000
}

enum LeashSymbol {
    static let mission = "rectangle.3.group"
    static let folder = "folder"
    static let undo = "arrow.uturn.backward"
    static let install = "square.and.arrow.down"
    static let quit = "power"
    static let alwaysList = "minus.circle"
    static let addFolder = "plus"
}

enum LeashCopy {
    static let app = "Leash"
    static let danger = "Danger"
    static let secret = "Secret"
    static let outside = "Outside"
    static let ask = "Ask"
    static let mission = "Mission Control"
    static let job = "Job"
    static let result = "Result"
    static let idle = "Idle"
    static let needsYou = "Needs you"
    static let watching = "Watching"
    static let offline = "Offline"
    static let plan = "Plan"
    static let act = "Act"
    static let review = "Review"
    static let fail = "Fail"
    static let tape = "Tape"
    static let working = "Working"
    static let done = "Done"
    static let failedLive = "Failed"
    static let steerPrompt = "Steer the agent…"
    static let noJob = "No job."
    static let emptyTape = "Nothing happened yet."
    static let caughtUp = "Caught up"
    static let noWaiting = "Nothing waiting."
    static let queued = "Queued"
    static let waitingOnYou = "Waiting on you"
    static let kill = "Kill"
    static let always = "Always"
    static let allow = "Allow"
    static let steer = "Steer"
    static let cut = "Cut"
    static let rewind = "Rewind"
    static let retry = "Retry"
    static let skip = "Skip"
    static let hintEsc = "esc"
    static let hintReturn = "↩"
    static let hintCommandReturn = "⌘↩"
    static let hintSteer = "⌘L"
    static let hintUndo = "⌘Z"
    static let hintRetry = "⌘R"
    static let hintSkip = "⌘."
    static let missionMenu = "Plan · act · review"
    static let pickFolder = "Pick a folder to protect"
    static let chooseFolders = "Choose the project folders"
    static let watchFolders = "Watch folders"
    static let addFolder = "Add folder"
    static let undoLastBurst = "Undo last burst"
    static let installHooks = "Install hooks"
    static let quitLeash = "Quit Leash"
    static let nothingToRestore = "Nothing to restore"
    static let lastBurst = "in the last burst"
    static let waitingOnYouA11y = "Leash — waiting on you"
    static let installAgents = "Cursor · Claude · Codex · OpenCode"
    static let agents = "Agents"
    static let noAgents = "No agents on this Mac"
    static let hooksTag = "hooks"
    static let acpTag = "ACP"
    static let addFolderPrompt = "Add a folder Leash should protect"
    static let helperMissing = "leash helper not found — run make install"
    static let installFailed = "install failed"
    static let hooksInstalled = "Hooks installed"
    static let couldNotStart = "Could not start leash serve"
    static let steering = "Steering"
    static let retryArmed = "Retry armed"
    static let alwaysRules = "Always"
    static let noAlways = "No always rules"
    static let revoked = "Revoked"
    static let unwatched = "Stopped watching"
    static let readyRewind = "Ready to rewind"
    static let dot = " · "
    static let reasons = "  ·  "

    static func needsYou(_ count: Int) -> String { "\(needsYou) · \(count)" }
    static func folders(_ count: Int) -> String { "\(count) folders" }
    static func files(_ n: Int) -> String { "\(n) file\(n == 1 ? "" : "s")" }
    static func filesIn(_ n: Int, folder: String) -> String { "\(files(n)) in \(folder)" }
    static func filesBurst(_ n: Int) -> String { "\(files(n)) \(lastBurst)" }
    static func moreWaiting(_ n: Int) -> String { n == 1 ? "1 more waiting." : "\(n) more waiting." }
    static func phaseTitle(_ phase: String, title: String) -> String { "\(phase) · \(title)" }
    static func andMore(_ n: Int) -> String { "+\(n) more" }
    static func restored(_ n: Int) -> String { "Restored \(files(n))" }
}

enum MissionPhase: String, CaseIterable, Identifiable {
    case idle, plan, act, review, failed

    var id: String { rawValue }

    init(_ raw: String?) {
        self = MissionPhase(rawValue: raw ?? "") ?? .idle
    }

    static let lights: [MissionPhase] = [.plan, .act, .review]

    var label: String {
        switch self {
        case .plan: return LeashCopy.plan
        case .act: return LeashCopy.act
        case .review: return LeashCopy.review
        case .failed: return LeashCopy.fail
        case .idle: return LeashCopy.idle
        }
    }

    var tint: Color {
        switch self {
        case .failed: return LeashPaint.vermillion
        case .act: return LeashPaint.moss
        case .plan: return LeashPaint.steel
        case .review: return LeashPaint.amber
        case .idle: return LeashPaint.muted
        }
    }

    var isLive: Bool { self == .act || self == .failed }

    func isLit(phase: String, waiting: Int) -> Bool {
        switch self {
        case .plan: return phase == rawValue
        case .act: return phase == rawValue || waiting > 0
        case .review: return phase == rawValue
        case .failed: return phase == rawValue
        case .idle: return phase == rawValue
        }
    }

    static func headerTint(phase: String, waiting: Int) -> Color {
        if Self(phase) == .failed || waiting > 0 { return LeashPaint.vermillion }
        return Self(phase).tint
    }
}

enum DaemonStatus: String {
    case watching, waiting, idle, offline

    init(_ raw: String?) {
        self = DaemonStatus(rawValue: raw ?? "") ?? .offline
    }
}

enum TapeKind: String {
    case plan, thought, steer, tool, undo, diff, gate, interrupt, error, skip

    init(_ raw: String) {
        self = TapeKind(rawValue: raw) ?? .thought
    }

    var label: String {
        switch self {
        case .plan: return LeashCopy.plan
        case .thought, .steer: return LeashCopy.steer
        case .skip: return LeashCopy.skip
        case .tool: return LeashCopy.act
        case .undo: return LeashCopy.rewind
        case .diff: return LeashCopy.result
        case .gate: return LeashCopy.waitingOnYou
        case .interrupt: return LeashCopy.kill
        case .error: return LeashCopy.fail
        }
    }

    var color: Color {
        switch self {
        case .plan, .diff: return LeashPaint.steel
        case .thought, .steer, .skip: return LeashPaint.muted
        case .tool, .undo: return LeashPaint.moss
        case .gate: return LeashPaint.amber
        case .interrupt, .error: return LeashPaint.vermillion
        }
    }
}

enum LiveStatus: String {
    case running, waiting, ok, error

    init(_ raw: String) {
        self = LiveStatus(rawValue: raw) ?? .running
    }

    var label: String {
        switch self {
        case .running: return LeashCopy.working
        case .waiting: return LeashCopy.waitingOnYou
        case .ok: return LeashCopy.done
        case .error: return LeashCopy.failedLive
        }
    }

    var tint: Color {
        switch self {
        case .error, .waiting: return LeashPaint.vermillion
        case .ok, .running: return LeashPaint.moss
        }
    }

    static func resultTint(_ result: String) -> Color {
        switch result {
        case "error", "deny": return LeashPaint.vermillion
        default: return LeashPaint.moss
        }
    }
}

enum LeashKind {
    case destroy, secret, outside, other

    init(_ raw: String) {
        switch raw {
        case "destroy": self = .destroy
        case "secret": self = .secret
        case "outside": self = .outside
        default: self = .other
        }
    }

    var label: String {
        switch self {
        case .destroy: return LeashCopy.danger
        case .secret: return LeashCopy.secret
        case .outside: return LeashCopy.outside
        case .other: return LeashCopy.ask
        }
    }

    var color: Color {
        switch self {
        case .destroy: return LeashPaint.vermillion
        case .secret: return LeashPaint.amber
        case .outside: return LeashPaint.steel
        case .other: return LeashPaint.ink.opacity(LeashPaint.Opacity.kindOther)
        }
    }
}

enum LeashHint {
    case paper, ink, vermillion
}

enum LeashFormat {
    static func duration(_ ms: Int) -> String {
        if ms < 1000 { return "\(ms)ms" }
        return String(format: "%.1fs", Double(ms) / 1000)
    }

    static func liveHeadline(_ live: LiveCall) -> String {
        if let outcome = live.outcome?.trimmingCharacters(in: .whitespacesAndNewlines), !outcome.isEmpty {
            return outcome
        }
        let detail = live.detail.trimmingCharacters(in: .whitespacesAndNewlines)
        if !detail.isEmpty { return detail }
        return live.tool
    }

    static func failedHeadline(_ failed: FailedCall) -> String {
        if let outcome = failed.outcome?.trimmingCharacters(in: .whitespacesAndNewlines), !outcome.isEmpty {
            return outcome
        }
        return failed.tool
    }

    static func pendingHeadline(_ pending: PendingApproval) -> String {
        let title = pending.title.trimmingCharacters(in: .whitespacesAndNewlines)
        if !title.isEmpty { return title }
        return pending.detail
    }

    static func compactPath(_ path: String) -> String {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        if path.hasPrefix(home) {
            return "~" + path.dropFirst(home.count)
        }
        return path
    }

    static func folderName(_ path: String) -> String {
        URL(fileURLWithPath: path).lastPathComponent
    }

    static func statusTitle(waiting: Int, offline: Bool, status: String) -> String {
        if waiting > 1 { return LeashCopy.needsYou(waiting) }
        if waiting == 1 { return LeashCopy.needsYou }
        if offline { return LeashCopy.offline }
        switch DaemonStatus(status) {
        case .watching: return LeashCopy.watching
        case .waiting: return LeashCopy.needsYou
        case .idle: return LeashCopy.idle
        case .offline: return LeashCopy.offline
        }
    }

    static func statusTint(waiting: Int, offline: Bool, status: String) -> Color {
        if waiting > 0 { return LeashPaint.vermillion }
        if offline { return LeashPaint.muted }
        if DaemonStatus(status) == .watching { return LeashPaint.moss }
        return LeashPaint.muted
    }

    static func statusDetail(pending: PendingApproval?, error: String?, folders: [String]) -> String {
        if let pending { return pendingHeadline(pending) }
        if let error { return error }
        if folders.count > 1 { return LeashCopy.folders(folders.count) }
        if let root = folders.first, !root.isEmpty { return compactPath(root) }
        return LeashCopy.pickFolder
    }

    static func missionSubtitle(phase: String, title: String?) -> String {
        if let title, !title.isEmpty, MissionPhase(phase) != .idle {
            return LeashCopy.phaseTitle(phase, title: title)
        }
        return LeashCopy.missionMenu
    }

    static func watchSubtitle(_ folders: [String]) -> String {
        if folders.isEmpty { return LeashCopy.chooseFolders }
        return folders.map(compactPath).joined(separator: LeashCopy.dot)
    }

    static func agentLine(_ agents: [AgentInfo]?) -> String? {
        let found = (agents ?? []).filter(\.installed)
        if found.isEmpty { return nil }
        return found.map { agent in
            "\(agent.name) \(agentTag(agent))"
        }.joined(separator: LeashCopy.dot)
    }

    static func installSubtitle(_ agents: [AgentInfo]?) -> String {
        let found = (agents ?? []).filter(\.installed)
        if found.isEmpty { return LeashCopy.installAgents }
        return found.map(\.name).joined(separator: LeashCopy.dot)
    }

    static func agentTag(_ agent: AgentInfo) -> String {
        if agent.hooked { return LeashCopy.hooksTag }
        if agent.door == "acp" || agent.door == "both" { return LeashCopy.acpTag }
        return "off"
    }

    static func undoSubtitle(_ burst: BurstInfo?) -> String {
        guard let burst else { return LeashCopy.nothingToRestore }
        let names = burst.files.prefix(3).map { folderName($0) }
        if !names.isEmpty {
            var line = names.joined(separator: LeashCopy.dot)
            let extra = burst.files.count - names.count
            if extra > 0 { line += LeashCopy.dot + LeashCopy.andMore(extra) }
            return line
        }
        if let root = burst.root, !root.isEmpty {
            return LeashCopy.filesIn(burst.fileCount, folder: folderName(root))
        }
        return LeashCopy.filesBurst(burst.fileCount)
    }

    static func alwaysSubtitle(_ rule: AlwaysRule) -> String {
        var parts: [String] = []
        if !rule.tool.isEmpty { parts.append(rule.tool) }
        if let root = rule.root, !root.isEmpty { parts.append(folderName(root)) }
        if parts.isEmpty { return LeashCopy.always }
        return parts.joined(separator: LeashCopy.dot)
    }

    static func waitingCall(_ pending: PendingApproval) -> LiveCall {
        LiveCall(
            tool: pending.tool,
            detail: pending.detail,
            outcome: pending.title,
            agent: pending.agent,
            root: pending.root,
            started: nil,
            status: LiveStatus.waiting.rawValue,
            durationMs: nil,
            result: nil,
            error: nil
        )
    }

    static func moreWaiting(total: Int) -> String? {
        let extra = total - 1
        guard extra > 0 else { return nil }
        return LeashCopy.moreWaiting(extra)
    }

    static func menuArmed(waiting: Int, phase: String?) -> Bool {
        waiting > 0 || MissionPhase(phase).isLive
    }

    static func markFilled(waiting: Int, status: String) -> Bool {
        waiting > 0 || DaemonStatus(status) == .watching
    }

    static func missionLive(phase: String?, pending: Bool) -> Bool {
        pending || MissionPhase(phase).isLive
    }

    static func headerMarkFilled(phase: String, waiting: Int) -> Bool {
        waiting > 0 || MissionPhase(phase).isLive
    }
}

enum LeashControlSize {
    case action, control

    var height: CGFloat {
        switch self {
        case .action: return LeashSpace.action
        case .control: return LeashSpace.control
        }
    }
}

enum LeashAction {
    case kill, always, allow, ghost, retry

    var fill: Color {
        switch self {
        case .kill: return LeashPaint.vermillion
        case .always, .ghost: return LeashPaint.faint
        case .allow, .retry: return LeashPaint.ink
        }
    }

    var ink: Color {
        switch self {
        case .kill: return LeashPaint.bone
        case .always, .ghost: return LeashPaint.ink
        case .allow, .retry: return LeashPaint.paper
        }
    }

    var hint: LeashHint {
        switch self {
        case .kill: return .vermillion
        case .always, .ghost: return .paper
        case .allow, .retry: return .ink
        }
    }

    var bordered: Bool { self == .always }
    var compactPad: Bool { self == .always || self == .ghost }
    var weight: Font.Weight { self == .always || self == .ghost ? .medium : .semibold }
}

extension Color {
    init(light: NSColor, dark: NSColor) {
        self.init(nsColor: NSColor(name: nil, dynamicProvider: { appearance in
            appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? dark : light
        }))
    }
}
