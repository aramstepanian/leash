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
  "cwd": "/absolute/project/root",
  "agent": "OpenCode",
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf ./dist" }
}
```

`tool_name` can be `Bash` / `Shell` / `bash`, `Write` / `write`, `Edit` / `edit`, `Read` / `read`. File tools should put the path in `tool_input.file_path` or `filePath`. Optional `agent` labels the HUD when several agents are running (`Cursor`, `Claude`, `Codex`, `OpenCode`).

Each hook is scoped to `cwd`: Always-allow, outside-project checks, and undo bursts are per folder, so two agents in two repos do not share a seatbelt.

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
