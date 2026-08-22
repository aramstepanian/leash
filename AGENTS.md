# AGENTS.md

## Cursor Cloud specific instructions

Leash is a pure-Go CLI + local daemon plus a macOS SwiftUI menu bar app.

### Scope in the cloud (Linux) VM
- Only the Go CLI/daemon (`cmd/leash`, `internal/...`) can be built, tested, and run here.
- The macOS app under `macos/` requires Xcode/Swift and cannot be built on Linux. `make assets` (`swift ...`) is also macOS-only.

### Build / lint / test / run
- Build: `make leash` (outputs `bin/leash`). The module is pure standard library (no external deps).
- Lint: `go vet ./...`.
- Test: `go test ./...` and `go test -race ./...` (both wired into `make test`).
- Run the daemon: `./bin/leash serve` (listens on `127.0.0.1:17332`). Run it as a long-lived process (e.g. a tmux-backed terminal), not inline, since it blocks.

### Runtime caveats (non-obvious)
- `leash serve` writes/reads `~/.leash/config.json` (port + auth token). Other subcommands (`status`, `watch`, `decide`, `hook`) read that file to reach the daemon, so the daemon must be running first.
- `leash hook` blocks (up to ~9 min) on a *dangerous* command until a decision arrives. To resolve it non-interactively, in another shell find the id via `leash status` and run `leash decide <id> allow|always|kill`. Safe commands (e.g. `git status`) return `{"decision":"allow"}` immediately.
- If the daemon is down, hooks fail open (return `{}` = allow), so a passing `leash hook` does not by itself prove the daemon is up — check `leash status`.
- `leash install` writes hook files into `~/.cursor`, `~/.claude`, `~/.codex`, `~/.config/opencode`. Avoid running it in the cloud VM unless you intend to modify those user configs.
