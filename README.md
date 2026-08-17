# Leash

<img src="docs/shots/icon.png" width="72" alt="Leash clip-and-strap mark">

Seatbelt for coding agents.

A Mac menu bar app that pops a native **Allow / Always / Kill** panel when a coding agent is about to run something dangerous, and can **undo the last burst of file changes**.

Works with **Cursor**, **OpenCode**, **Claude Code**, **Codex**, and any custom agent that can call a hook. Local only. No account. No model.

<p align="center">
  <img src="docs/shots/approval.png" width="432" alt="Leash approval panel for rm -rf ./dist">
</p>

## What it does

1. Installs hooks (or a tiny plugin) into the agents you use.
2. Safe calls pass through with no UI (`git status`, reads, normal edits).
3. Dangerous calls wait on a Mac panel: `rm -rf`, `sudo`, `curl | sh`, force-push, `.env`, writes outside the project.
4. Before a mutating call, Leash snapshots the files it can see.
5. **Undo last burst** puts those files back.

## Quick start (Mac)

You need Go 1.22+ and Xcode 15+ (macOS 14).

```bash
make install          # builds ~/.leash/bin/leash and writes hooks
open macos/Leash.xcodeproj
```

Run the **Leash** target. A clip-and-strap mark appears in the menu bar.

1. **Watch folders** — pick the repos you vibe-code in. Leash also learns a folder when an agent first runs there.
2. Leave Leash running. It starts the local daemon for you.
3. Use Cursor, OpenCode, `claude`, or `codex` as usual — even several at once, in several folders.
4. When something scary is about to run, the panel appears.
   - **Kill** — Esc
   - **Allow** — Return
   - **Always** — ⌘Return (exact command/path match in that folder, not a prefix)

Record the demo without waiting for a real agent:

```bash
~/.leash/bin/leash demo
```

That fakes a `Bash` call of `rm -rf ./dist` and blocks until you decide.

## CLI (no UI)

The engine is a local daemon on `127.0.0.1:17332`.

```bash
make leash
./bin/leash serve          # in one terminal
./bin/leash install
./bin/leash watch .
./bin/leash demo           # blocks until you decide
# other terminal:
./bin/leash status
./bin/leash decide <id> kill
./bin/leash undo
```

If the daemon is down, hooks **fail open** so agents keep working.

## Agents

| Agent | Install target |
|---|---|
| Cursor | `~/.cursor/hooks.json` |
| OpenCode | `~/.config/opencode/plugins/leash.js` |
| Claude Code | `~/.claude/settings.json` |
| Codex | `~/.codex/hooks.json` |
| Custom / work agent | [docs/INTEGRATION.md](docs/INTEGRATION.md) |

Home-grown agents: POST the same JSON to `http://127.0.0.1:17332/v1/hook` or spawn `leash hook`. That is the path for a private work agent without forking Leash.

## What gets a prompt

| Kind | Examples |
|---|---|
| Danger | `rm -rf`, `sudo`, `curl \| sh`, `git push --force`, `git reset --hard`, `chmod 777`, db reset |
| Secret | `.env`, `id_rsa`, `*.pem`, `.npmrc`, `~/.aws/credentials` |
| Outside | write/edit path outside that project's watched folder |

Normal source edits and read-only commands do **not** prompt. They still snapshot when they mutate, so undo works. Undo restores the last burst in the folder that was touched most recently — two projects stay separate.

## Layout

```
cmd/leash/           CLI + daemon
internal/policy/     allow vs ask
internal/burst/      file snapshot + undo
internal/server/     localhost HTTP
internal/install/    Cursor, Claude, Codex, OpenCode wiring
docs/INTEGRATION.md  protocol for any other agent
macos/Leash/         SwiftUI menu bar + approval panel
```

## 15-second clip

Mute. One take.

1. Terminal: agent wants `rm -rf` or `cat .env`
2. Native panel, Kill
3. Menu bar: undo last burst (optional)

Caption: *Seatbelt for coding agents. Local. Allow / Kill / Undo.*

## Notes

- Codex hooks are opt-in (`codex_hooks = true` in `~/.codex/config.toml`). Leash adds that line. You may still need `/hooks` in Codex to trust the command.
- Not a sandbox. The agent still runs as you. This is an interrupt + rewind, not a VM.
- OpenCode loads `~/.config/opencode/plugins/` automatically; restart OpenCode after `leash install`.
