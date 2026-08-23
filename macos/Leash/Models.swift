import Foundation

struct LeashState: Codable, Equatable {
    var status: String
    var watchRoot: String?
    var watchRoots: [String]?
    var pending: PendingApproval?
    var queue: [PendingApproval]?
    var waiting: Int?
    var burst: BurstInfo?
    var lastKill: Date?
    var alwaysAllow: [AlwaysRule]
    var port: Int?
    var mission: MissionInfo?
    var agents: [AgentInfo]?
    var job: JobInfo?
    var version: String?

    static let empty = LeashState(
        status: "offline",
        watchRoot: nil,
        watchRoots: nil,
        pending: nil,
        queue: nil,
        waiting: nil,
        burst: nil,
        lastKill: nil,
        alwaysAllow: [],
        port: nil,
        mission: nil,
        agents: nil,
        job: nil,
        version: nil
    )

    var folders: [String] {
        if let roots = watchRoots, !roots.isEmpty { return roots }
        if let r = watchRoot, !r.isEmpty { return [r] }
        return []
    }

    var allPending: [PendingApproval] {
        var out: [PendingApproval] = []
        if let p = pending { out.append(p) }
        if let q = queue { out.append(contentsOf: q) }
        return out
    }

    var waitingCount: Int {
        if let w = waiting, w > 0 { return w }
        return allPending.count
    }

    var phase: String { mission?.phase ?? status }
}

struct MissionInfo: Codable, Equatable {
    var phase: String
    var title: String
    var goal: String?
    var agent: String?
    var root: String?
    var live: LiveCall?
    var failed: FailedCall?
    var steer: String?
    var timeline: [TimelineEvent]
}

struct LiveCall: Codable, Equatable {
    var tool: String
    var detail: String
    var outcome: String?
    var agent: String?
    var root: String?
    var started: Date?
    var status: String
    var durationMs: Int?
    var result: String?
    var error: String?
}

struct FailedCall: Codable, Equatable {
    var tool: String
    var detail: String
    var outcome: String?
    var error: String
    var agent: String?
}

struct TimelineEvent: Codable, Equatable, Identifiable {
    var id: String
    var at: Date?
    var kind: String
    var agent: String?
    var tool: String?
    var title: String
    var detail: String?
    var result: String?
    var error: String?
    var durationMs: Int?
    var paths: [String]?
    var root: String?
}

struct PendingApproval: Codable, Equatable, Identifiable {
    var id: String
    var tool: String
    var title: String
    var detail: String
    var kind: String
    var reasons: [String]
    var pattern: String?
    var cwd: String?
    var agent: String?
    var root: String?
}

struct BurstInfo: Codable, Equatable {
    var id: String
    var started: Date?
    var fileCount: Int
    var files: [String]
    var root: String?
}

struct AlwaysRule: Codable, Equatable, Identifiable {
    var tool: String
    var pattern: String
    var root: String?

    var id: String { "\(tool)|\(pattern)|\(root ?? "")" }
}

struct UndoResponse: Codable {
    var restored: Int
}

struct AgentInfo: Codable, Equatable, Identifiable {
    var id: String
    var name: String
    var installed: Bool
    var hooked: Bool
    var door: String
    var path: String?
    var acp: String?
}

struct JobInfo: Codable, Equatable {
    var prompt: String
    var agent: String?
    var root: String?
    var status: String
    var error: String?
    var result: String?

    var running: Bool { status == "running" }

    var displayText: String {
        let err = error?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if status == "failed", !err.isEmpty { return err }
        return result?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }
}
