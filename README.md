# Leash

<img src="docs/shots/icon.png" width="72" alt="Leash clip-and-strap mark">

Dispatch a prompt to an installed CLI agent. No agent UI, no approvals, no Mission Control.

One field. Pick a folder. Return. The agent runs in the background; status is idle / running / done / failed, and the result comes back in the menu.

Works with **OpenCode**, **Claude Code**, **Cursor CLI**, **Codex**, **Hermes**, and **Grok**. Cursor.app is GUI-only, so it is skipped. Local only. No account. No model of Leash’s own.

## Quick start (Mac)

You need Go 1.22+ and Xcode 15+ (macOS 14).

```bash
make install          # builds ~/.leash/bin/leash
open macos/Leash.xcodeproj
```

Run the **Leash** target. A clip-and-strap mark appears in the menu bar.

1. Pick the project folder.
2. Type what the agent should do. Press Return.
3. Wait. The mark pulses while a job is running. Done shows the result.

Leash starts the local daemon for you. On this Mac, OpenCode is the default pick when it is installed.

## CLI (no UI)

The engine is a local daemon on `127.0.0.1:17332`.

```bash
make leash
./bin/leash serve          # in one terminal
./bin/leash install
./bin/leash watch .
./bin/leash run "fix the flaky test"
./bin/leash run --agent claude -- "explain cmd/leash/main.go"
./bin/leash status
```

Or after `make install`:

```bash
~/.leash/bin/leash run "fix the flaky test"
```

One job at a time. A second `run` while one is in flight returns 409.

## Agents

Leash does not open the agent’s UI. It finds a CLI on your PATH (`leash status`) and sends the prompt headless.

| Agent | How |
|---|---|
| OpenCode | `opencode run` (preferred default) |
| Claude Code | `claude -p` with permissions bypassed |
| Cursor CLI | `cursor-agent -p --print` |
| Codex | `codex exec --full-auto` |
| Hermes / Grok | ACP one-shot (`hermes acp` / `grok agent stdio`) |

Cursor.app has no headless prompt, so it is not a dispatch target. Hooked agents still run as usual; Leash auto-allows every tool call (no Kill / Allow panel).

## Layout

```
cmd/leash/           CLI + daemon
internal/dispatch/   pick a CLI, run one prompt
internal/server/     localhost HTTP, including POST /v1/run
internal/agents/     which CLIs are on this Mac
internal/acp/        ACP stdio helper
macos/Leash/         SwiftUI menu: prompt, folder, status, quit
```

## Notes

- Not a sandbox. The agent still runs as you.
- OpenCode loads `~/.config/opencode/plugins/` automatically; restart OpenCode after `leash install` if you still want hooks (they auto-allow).
- Codex hooks are opt-in (`codex_hooks = true` in `~/.codex/config.toml`).
