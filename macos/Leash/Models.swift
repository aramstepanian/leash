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
        waiting: 0,
        burst: nil,
        lastKill: nil,
        alwaysAllow: [],
        port: nil
    )

    var waitingCount: Int {
        if let waiting { return waiting }
        return pending == nil ? 0 : 1
    }

    var folders: [String] {
        if let watchRoots, !watchRoots.isEmpty { return watchRoots }
        if let watchRoot, !watchRoot.isEmpty { return [watchRoot] }
        return []
    }

    var allPending: [PendingApproval] {
        var rows: [PendingApproval] = []
        if let pending { rows.append(pending) }
        if let queue { rows.append(contentsOf: queue) }
        return rows
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
    var root: String?
    var agent: String?
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
