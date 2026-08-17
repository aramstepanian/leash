import Foundation

struct DaemonClient {
    private var base: URL {
        URL(string: "http://127.0.0.1:17332")!
    }

    private var token: String {
        tokenFromConfig() ?? ""
    }

    func reachable() -> Bool {
        var req = URLRequest(url: base.appendingPathComponent("v1/health"))
        req.timeoutInterval = 0.4
        let sem = DispatchSemaphore(value: 0)
        var ok = false
        URLSession.shared.dataTask(with: req) { _, res, _ in
            ok = (res as? HTTPURLResponse)?.statusCode == 200
            sem.signal()
        }.resume()
        _ = sem.wait(timeout: .now() + 0.5)
        return ok
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

    func watch(_ path: String) async throws {
        var req = try authorized("POST", "/v1/watch")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["path": path])
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

func tokenFromConfig() -> String? {
    let home = FileManager.default.homeDirectoryForCurrentUser
    let url = home.appendingPathComponent(".leash/config.json")
    guard let data = try? Data(contentsOf: url),
          let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
          let token = obj["token"] as? String
    else { return nil }
    return token
}
