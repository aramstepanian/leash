import Foundation

struct LeashConfig {
    var port: Int
    var token: String

    static func load() -> LeashConfig {
        let home = FileManager.default.homeDirectoryForCurrentUser
        let url = home.appendingPathComponent(".leash/config.json")
        guard let data = try? Data(contentsOf: url),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else {
            return LeashConfig(port: 17332, token: "")
        }
        let port: Int
        if let n = obj["port"] as? Int {
            port = n
        } else if let n = obj["port"] as? NSNumber {
            port = n.intValue
        } else {
            port = 17332
        }
        return LeashConfig(port: port == 0 ? 17332 : port, token: obj["token"] as? String ?? "")
    }
}

struct DaemonClient {
    private var config: LeashConfig { LeashConfig.load() }

    private var base: URL {
        URL(string: "http://127.0.0.1:\(config.port)")!
    }

    private var token: String { config.token }

    func reachable() async -> Bool {
        var req = URLRequest(url: base.appendingPathComponent("v1/health"))
        req.timeoutInterval = 0.4
        do {
            let (_, res) = try await URLSession.shared.data(for: req)
            return (res as? HTTPURLResponse)?.statusCode == 200
        } catch {
            return false
        }
    }

    func state() async throws -> LeashState {
        var req = try authorized("GET", "/v1/state")
        req.timeoutInterval = 1
        let (data, res) = try await URLSession.shared.data(for: req)
        try check(res)
        let dec = JSONDecoder()
        dec.dateDecodingStrategy = .iso8601
        return try dec.decode(LeashState.self, from: data)
    }

    func decide(id: String, action: String) async throws {
        var req = try authorized("POST", "/v1/decision")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["id": id, "action": action])
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func undo() async throws -> Int {
        var req = try authorized("POST", "/v1/undo")
        req.httpBody = Data("{}".utf8)
        let (data, res) = try await URLSession.shared.data(for: req)
        try check(res)
        return try JSONDecoder().decode(UndoResponse.self, from: data).restored
    }

    func watch(_ path: String, remove: Bool = false) async throws {
        var req = try authorized("POST", "/v1/watch")
        var body: [String: Any] = ["path": path]
        if remove {
            body["remove"] = true
        }
        req.httpBody = try JSONSerialization.data(withJSONObject: body)
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func steer(_ text: String) async throws {
        var req = try authorized("POST", "/v1/steer")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["text": text])
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func interrupt(_ text: String) async throws {
        var req = try authorized("POST", "/v1/interrupt")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["text": text])
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func retry() async throws {
        var req = try authorized("POST", "/v1/retry")
        req.httpBody = Data("{}".utf8)
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func skip() async throws {
        var req = try authorized("POST", "/v1/skip")
        req.httpBody = Data("{}".utf8)
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    func unwatch(_ path: String) async throws {
        try await watch(path, remove: true)
    }

    func revokeAlways(_ rule: AlwaysRule) async throws {
        var req = try authorized("POST", "/v1/always")
        var body: [String: Any] = [
            "remove": true,
            "tool": rule.tool,
            "pattern": rule.pattern,
        ]
        if let root = rule.root, !root.isEmpty {
            body["root"] = root
        }
        req.httpBody = try JSONSerialization.data(withJSONObject: body)
        let (_, res) = try await URLSession.shared.data(for: req)
        try check(res)
    }

    private func authorized(_ method: String, _ path: String) throws -> URLRequest {
        var req = URLRequest(url: base.appendingPathComponent(String(path.dropFirst())))
        req.httpMethod = method
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        return req
    }

    private func check(_ res: URLResponse) throws {
        let code = (res as? HTTPURLResponse)?.statusCode ?? 0
        if code >= 400 {
            throw URLError(.badServerResponse)
        }
    }
}
