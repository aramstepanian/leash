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
        port: nil
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

struct AlwaysRule: Codable, Equatable {
    var tool: String
    var pattern: String
    var root: String?
}

struct UndoResponse: Codable {
    var restored: Int
}
