# Leash

Seatbelt for coding agents.

A Mac menu bar app that pops a native **Allow / Always / Kill** sheet when Claude Code or Codex is about to run something dangerous, and can **undo the last burst of file changes**.

Local only. No account. No model. You already have the agent — this is the brake.

## What it does

1. Hooks into Claude Code and Codex (`PreToolUse`).
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

Run the **Leash** target. The menu bar icon appears (link / raised hand).

1. **Watch folder…** — pick the repo you vibe-code in.
2. Leave Leash running.
3. In a terminal: `claude` or `codex` as usual.
4. When something scary is about to run, the sheet appears.
   - **Kill** — Esc or ⌘.
   - **Allow** — Return
   - **Always** — ⌘Return (saves a rule)

Record the demo without waiting for a real agent:

```bash
~/.leash/bin/leash demo 'rm -rf ./dist'
```

## CLI (no UI)

The engine is a local daemon on `127.0.0.1:17332`.

```bash
make leash
./bin/leash serve          # in one terminal
./bin/leash install
./bin/leash watch .
./bin/leash demo 'rm -rf ./dist'   # blocks until you decide
# other terminal:
./bin/leash status
./bin/leash decide <id> kill
./bin/leash undo
```

If the daemon is down, hooks **fail open** (empty `{}`) so Claude/Codex keep working.

## What gets a prompt

| Kind | Examples |
|---|---|
| Danger | `rm -rf`, `sudo`, `curl \| sh`, `git push --force`, `git reset --hard`, `chmod 777`, db reset |
| Secret | `.env`, `id_rsa`, `*.pem`, `.npmrc`, `~/.aws/credentials` |
| Outside | write/edit path outside the watched folder |

Normal source edits and read-only commands do **not** prompt. They still snapshot when they mutate, so undo works.

## Layout

```
cmd/leash/           CLI + daemon
internal/policy/     allow vs ask
internal/burst/      file snapshot + undo
internal/server/     localhost HTTP
internal/install/    ~/.claude + ~/.codex hooks
macos/Leash/         SwiftUI menu bar + approval panel
```

## 15-second clip

Mute. One take.

1. Terminal: agent wants `rm -rf` or `cat .env`
2. Native sheet, Kill
3. Menu bar: undo last burst (optional)

Caption: *Seatbelt for coding agents. Local. Allow / Kill / Undo.*

## Notes

- Codex hooks are opt-in (`codex_hooks = true` in `~/.codex/config.toml`). Leash adds that line. You may still need `/hooks` in Codex to trust the command.
- Not a sandbox. The agent still runs as you. This is an interrupt + rewind, not a VM.
- v1 does not parse OpenCode yet.
