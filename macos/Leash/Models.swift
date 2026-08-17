import Foundation

struct LeashState: Codable, Equatable {
    var status: String
    var watchRoot: String?
    var pending: PendingApproval?
    var burst: BurstInfo?
    var lastKill: Date?
    var alwaysAllow: [AlwaysRule]
    var port: Int?

    static let empty = LeashState(status: "offline", watchRoot: nil, pending: nil, burst: nil, lastKill: nil, alwaysAllow: [], port: nil)
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
}

struct BurstInfo: Codable, Equatable {
    var id: String
    var started: Date?
    var fileCount: Int
    var files: [String]
}

struct AlwaysRule: Codable, Equatable {
    var tool: String
    var pattern: String
}

struct UndoResponse: Codable {
    var restored: Int
}
