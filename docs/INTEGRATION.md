# Plug Leash into any agent

Leash is agent-agnostic. The Mac app / `leash serve` daemon is the seatbelt. Each coding agent only needs to call it **before a tool runs**, wait, then honor allow or deny.

`leash install` wires the ones we know. A home-grown agent uses the same JSON.

## What `leash install` writes

| Agent | Where | How |
|---|---|---|
| Cursor | `~/.cursor/hooks.json` | `preToolUse` + `beforeShellExecution` → `leash hook` |
| Claude Code | `~/.claude/settings.json` | `PreToolUse` command hook |
| Codex | `~/.codex/hooks.json` | `PreToolUse` + `PermissionRequest` |
| OpenCode | `~/.config/opencode/plugins/leash.js` | `tool.execute.before` spawns `leash hook` |

Keep `leash serve` (or Leash.app) running. If the daemon is down, hooks **fail open** so the agent still works.

Cursor may fire both `preToolUse` and `beforeShellExecution` for the same command. The daemon remembers the last decision for 3 seconds so you do not get two sheets.

Codex may ask you to trust the hook once via `/hooks`.

## Custom / work agent

Two equivalent APIs. Pick one.

### 1. Subprocess (simplest)

Before each tool call, spawn:

```bash
leash hook
```

Write JSON to stdin, read JSON from stdout. Exit 0.

**Request**

```json
{
  "protocol": "leash",
  "hook_event_name": "pre_tool",
  "agent": "MyAgent",
  "cwd": "/absolute/project/root",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf ./dist" }
}
```

`tool_name` can be `Bash` / `Shell` / `bash`, `Write` / `write`, `Edit` / `edit`, `Read` / `read`. File tools should put the path in `tool_input.file_path` or `filePath`.

Optional `agent` is a short label for the Mac panel (`OpenCode`, `Codex`, your work agent's name). Cursor and Claude are inferred from the hook dialect if you omit it.

Mission Control also accepts:

```json
{ "protocol": "leash", "hook_event_name": "plan", "cwd": "/repo", "text": "Fix login", "steps": ["read", "edit", "test"] }
```

```json
{ "protocol": "leash", "hook_event_name": "thought", "cwd": "/repo", "text": "checking middleware" }
```

```json
{ "protocol": "leash", "hook_event_name": "post_tool", "cwd": "/repo", "tool_name": "Bash", "tool_input": { "command": "npm test" }, "error": "exit status 1", "duration_ms": 1420 }
```

`leash install` already wires post-tool hooks for Cursor, Claude, and OpenCode so the inspector can show args, result, and duration. Custom agents should send `post_tool` after the call, not only `pre_tool`.

Steer / interrupt / retry are HTTP:

```
POST /v1/steer       { "text": "use bun, not npm" }
POST /v1/interrupt   { "text": "stop" }
POST /v1/retry       {}
POST /v1/skip        {}
```

A pending steer is injected as additional context on the next tool (Claude `additionalContext`, Cursor `agent_message`). Interrupt denies the current or next tool. Retry writes a steer note from the last tool error. None of this runs a second agent.

**Always** is scoped to the project folder Leash matched for `cwd`. A rule saved in one repo does not silently allow the same command in another. Older rules with no `root` still match everywhere.

`cwd` picks the project: the most specific watched folder that contains it, or `cwd` itself. Unknown project folders are added to the watch list automatically (not `$HOME` or `/`, and not a nested directory of a folder you already watch).

Undo restores the last burst in the folder that was touched most recently. Two agents in two repos do not share a rewind.

**Response**

```json
{ "decision": "allow", "reason": "Allowed by Leash" }
```

```json
{ "decision": "deny", "reason": "Blocked by Leash: rm -rf" }
```

If stdout is `{}` or the process fails, treat as **allow** (fail open).

The hook **blocks** until the user hits Allow / Always / Kill, up to ~8 minutes.

### 2. HTTP

`POST http://127.0.0.1:17332/v1/hook`

Headers:

```
Authorization: Bearer <token from ~/.leash/config.json>
Content-Type: application/json
```

Body and response are the same JSON as above.

Python:

```python
import json, urllib.request

cfg = json.load(open(Path.home() / ".leash" / "config.json"))
req = urllib.request.Request(
    f"http://127.0.0.1:{cfg['port']}/v1/hook",
    data=json.dumps({
        "protocol": "leash",
        "hook_event_name": "pre_tool",
        "cwd": cwd,
        "tool_name": tool,
        "tool_input": args,
    }).encode(),
    headers={
        "Authorization": f"Bearer {cfg['token']}",
        "Content-Type": "application/json",
    },
    method="POST",
)
out = json.load(urllib.request.urlopen(req, timeout=540))
if out.get("decision") == "deny":
    raise PermissionError(out.get("reason", "Blocked by Leash"))
```

Do this **before** the tool runs, not after. Undo snapshots the file at this moment.

## Do not send file contents

For reads, send the **path** only. Leash does not need `.env` bytes, and you should not log them.

## Check it

```bash
leash serve
leash watch /path/to/repo
echo '{"protocol":"leash","hook_event_name":"pre_tool","cwd":"/path/to/repo","tool_name":"Bash","tool_input":{"command":"rm -rf ./dist"}}' | leash hook
```

The native sheet should appear. Kill should print `"decision":"deny"`.
