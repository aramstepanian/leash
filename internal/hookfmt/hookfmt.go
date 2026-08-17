package hookfmt

import (
	"encoding/json"
	"maps"
	"strings"
)

type Dialect string

const (
	DialectClaude  Dialect = "claude"
	DialectCursor  Dialect = "cursor"
	DialectGeneric Dialect = "generic"
)

// Event is a normalized pre-tool event from any supported agent.
type Event struct {
	SessionID     string
	CWD           string
	HookEventName string
	ToolName      string
	ToolInput     map[string]any
	Dialect       Dialect
}

func Parse(data []byte) (Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Event{}, err
	}
	ev := Event{
		SessionID:     firstStr(raw, "session_id", "conversation_id"),
		CWD:           firstStr(raw, "cwd", "cwd_path"),
		HookEventName: firstStr(raw, "hook_event_name", "hookEventName"),
		ToolName:      firstStr(raw, "tool_name", "tool"),
		Dialect:       detectDialect(raw),
	}
	if ev.CWD == "" {
		if roots, ok := raw["workspace_roots"].([]any); ok && len(roots) > 0 {
			ev.CWD, _ = roots[0].(string)
		}
	}
	ev.ToolInput = map[string]any{}
	switch v := raw["tool_input"].(type) {
	case map[string]any:
		ev.ToolInput = maps.Clone(v)
	case string:
		var m map[string]any
		if json.Unmarshal([]byte(v), &m) == nil {
			ev.ToolInput = m
		} else if v != "" {
			ev.ToolInput["command"] = v
		}
	}
	normalizeCursorShape(raw, &ev)
	stripSecrets(ev.ToolInput)
	if wd, ok := ev.ToolInput["working_directory"].(string); ok && ev.CWD == "" {
		ev.CWD = wd
	}
	if ev.ToolName == "" {
		ev.ToolName = "Bash"
	}
	return ev, nil
}

func detectDialect(raw map[string]any) Dialect {
	if proto, _ := raw["protocol"].(string); strings.EqualFold(proto, "leash") {
		return DialectGeneric
	}
	name := firstStr(raw, "hook_event_name", "hookEventName")
	switch name {
	case "preToolUse", "beforeShellExecution", "beforeMCPExecution", "beforeReadFile":
		return DialectCursor
	case "pre_tool", "leash.pre_tool":
		return DialectGeneric
	case "PreToolUse", "PermissionRequest":
		return DialectClaude
	}
	if raw["cursor_version"] != nil || raw["conversation_id"] != nil {
		return DialectCursor
	}
	return DialectClaude
}

func normalizeCursorShape(raw map[string]any, ev *Event) {
	name := strings.ToLower(ev.HookEventName)
	switch name {
	case "beforeshellexecution":
		ev.ToolName = "Bash"
		if ev.ToolInput["command"] == nil {
			if cmd := firstStr(raw, "command"); cmd != "" {
				ev.ToolInput["command"] = cmd
			}
		}
	case "beforereadfile":
		ev.ToolName = "Read"
		if p := firstStr(raw, "file_path"); p != "" {
			ev.ToolInput["file_path"] = p
		}
	case "beforemcpexecution":
		if ev.ToolName == "" {
			ev.ToolName = firstStr(raw, "tool_name")
		}
		if cmd := firstStr(raw, "command"); cmd != "" && ev.ToolInput["command"] == nil {
			ev.ToolInput["command"] = cmd
		}
	}
	if strings.EqualFold(ev.ToolName, "Shell") {
		ev.ToolName = "Bash"
	}
}

func stripSecrets(input map[string]any) {
	if input == nil {
		return
	}
	delete(input, "content")
	delete(input, "contents")
	delete(input, "new_string")
	delete(input, "old_string")
	delete(input, "attachments")
}

func firstStr(raw map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := raw[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func IsPre(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	switch n {
	case "pretooluse", "permissionrequest", "beforeshellexecution", "beforemcpexecution", "beforereadfile", "pre_tool", "leash.pre_tool", "":
		return true
	default:
		return false
	}
}

func IsPermissionRequest(ev Event) bool {
	return strings.EqualFold(ev.HookEventName, "PermissionRequest")
}

type Decision string

const (
	DecisionAllow  Decision = "allow"
	DecisionAlways Decision = "always"
	DecisionKill   Decision = "kill"
)

func Encode(ev Event, d Decision, reason string) []byte {
	if d == DecisionAlways {
		d = DecisionAllow
	}
	switch ev.Dialect {
	case DialectCursor:
		return encodeCursor(d, reason)
	case DialectGeneric:
		return encodeGeneric(d, reason)
	default:
		return encodeClaude(ev, d, reason)
	}
}

func SilentAllow(ev Event) []byte {
	switch ev.Dialect {
	case DialectGeneric:
		return encodeGeneric(DecisionAllow, "Allowed by Leash")
	default:
		return []byte("{}\n")
	}
}

func encodeClaude(ev Event, d Decision, reason string) []byte {
	if IsPermissionRequest(ev) {
		behavior := "allow"
		out := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName": "PermissionRequest",
				"decision":      map[string]any{"behavior": behavior},
			},
		}
		if d == DecisionKill {
			out = map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName": "PermissionRequest",
					"decision": map[string]any{
						"behavior": "deny",
						"message":  reason,
					},
				},
			}
		}
		b, _ := json.Marshal(out)
		return b
	}
	perm := "allow"
	if d == DecisionKill {
		perm = "deny"
	}
	if reason == "" {
		if perm == "deny" {
			reason = "Blocked by Leash"
		} else {
			reason = "Allowed by Leash"
		}
	}
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       perm,
			"permissionDecisionReason": reason,
		},
	}
	b, _ := json.Marshal(out)
	return b
}

func encodeCursor(d Decision, reason string) []byte {
	perm := "allow"
	if d == DecisionKill {
		perm = "deny"
	}
	if reason == "" {
		if perm == "deny" {
			reason = "Blocked by Leash"
		} else {
			reason = "Allowed by Leash"
		}
	}
	out := map[string]any{
		"permission":    perm,
		"user_message":  reason,
		"agent_message": reason,
	}
	b, _ := json.Marshal(out)
	return append(b, '\n')
}

func encodeGeneric(d Decision, reason string) []byte {
	dec := "allow"
	if d == DecisionKill {
		dec = "deny"
	}
	if reason == "" {
		if dec == "deny" {
			reason = "Blocked by Leash"
		} else {
			reason = "Allowed by Leash"
		}
	}
	out := map[string]any{
		"decision": dec,
		"reason":   reason,
	}
	b, _ := json.Marshal(out)
	return append(b, '\n')
}
