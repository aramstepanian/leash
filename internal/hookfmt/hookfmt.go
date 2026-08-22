package hookfmt

import (
	"encoding/json"
	"maps"
	"os"
	"strings"
)

type Dialect string

const (
	DialectClaude  Dialect = "claude"
	DialectCursor  Dialect = "cursor"
	DialectGeneric Dialect = "generic"
)

// Event is a normalized hook from any supported agent.
type Event struct {
	SessionID     string
	CWD           string
	HookEventName string
	ToolName      string
	ToolInput     map[string]any
	Dialect       Dialect
	Agent         string
	Text          string // plan / thought / error / tool output
	DurationMs    int
	Steps         []string
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
		Agent:         firstStr(raw, "agent"),
	}
	if ev.Agent == "" {
		ev.Agent = strings.TrimSpace(os.Getenv("LEASH_AGENT"))
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
	fillResult(raw, &ev)
	if ev.ToolName == "" && (IsPre(ev) || IsPost(ev) || IsFailure(ev)) {
		ev.ToolName = "Bash"
	}
	return ev, nil
}

func fillResult(raw map[string]any, ev *Event) {
	ev.Text = firstStr(raw, "thought", "text", "message", "plan", "goal", "title", "error", "tool_error")
	if n, ok := asInt(raw["duration_ms"]); ok {
		ev.DurationMs = n
	} else if n, ok := asInt(raw["durationMs"]); ok {
		ev.DurationMs = n
	}
	if steps, ok := raw["steps"].([]any); ok {
		for _, s := range steps {
			if t, ok := s.(string); ok && t != "" {
				ev.Steps = append(ev.Steps, t)
			}
		}
	}
	out := firstStr(raw, "tool_output", "output", "result")
	if out == "" {
		switch v := raw["tool_response"].(type) {
		case string:
			out = v
		case map[string]any:
			out = firstStr(v, "stdout", "output", "result", "content")
			if ev.Text == "" {
				ev.Text = firstStr(v, "stderr", "error", "message")
			}
			stripSecrets(v)
		}
	}
	if ev.Text == "" {
		ev.Text = out
	} else if out != "" && ev.Text != out {
		if IsPost(*ev) || IsFailure(*ev) {
			if ev.Text == "" {
				ev.Text = out
			}
		}
	}
	if IsPost(*ev) || IsFailure(*ev) {
		if errText := firstStr(raw, "error", "tool_error", "stderr"); errText != "" {
			ev.Text = errText
		} else if out != "" && !IsFailure(*ev) {
			ev.Text = clip(out, 2000)
		}
		ev.Text = clip(ev.Text, 2000)
	}
	if IsPlan(*ev) || IsThought(*ev) {
		ev.Text = clip(firstNonEmpty(ev.Text, firstStr(raw, "thought", "text", "message", "plan", "goal")), 4000)
	}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func detectDialect(raw map[string]any) Dialect {
	if proto, _ := raw["protocol"].(string); strings.EqualFold(proto, "leash") {
		return DialectGeneric
	}
	name := firstStr(raw, "hook_event_name", "hookEventName")
	switch name {
	case "preToolUse", "postToolUse", "postToolUseFailure", "beforeShellExecution", "afterShellExecution", "beforeMCPExecution", "afterMCPExecution", "beforeReadFile", "afterFileEdit":
		return DialectCursor
	case "pre_tool", "post_tool", "leash.pre_tool", "leash.post_tool", "leash.plan", "leash.thought", "plan", "thought":
		return DialectGeneric
	case "PreToolUse", "PostToolUse", "PostToolUseFailure", "PermissionRequest":
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
	case "beforeshellexecution", "aftershellexecution":
		ev.ToolName = "Bash"
		if ev.ToolInput["command"] == nil {
			if cmd := firstStr(raw, "command"); cmd != "" {
				ev.ToolInput["command"] = cmd
			}
		}
		if ev.Text == "" {
			ev.Text = firstStr(raw, "output", "command_output", "stderr")
		}
	case "beforereadfile":
		ev.ToolName = "Read"
		if p := firstStr(raw, "file_path"); p != "" {
			ev.ToolInput["file_path"] = p
		}
	case "beforemcpexecution", "aftermcpexecution":
		if ev.ToolName == "" {
			ev.ToolName = firstStr(raw, "tool_name")
		}
		if cmd := firstStr(raw, "command"); cmd != "" && ev.ToolInput["command"] == nil {
			ev.ToolInput["command"] = cmd
		}
	case "afterfileedit":
		if ev.ToolName == "" {
			ev.ToolName = "Edit"
		}
		if p := firstStr(raw, "file_path", "path"); p != "" && ev.ToolInput["file_path"] == nil {
			ev.ToolInput["file_path"] = p
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

// AgentLabel is a short name for the HUD: Cursor, Claude, OpenCode, or the payload's agent field.
func AgentLabel(ev Event) string {
	if s := strings.TrimSpace(ev.Agent); s != "" {
		return s
	}
	switch ev.Dialect {
	case DialectCursor:
		return "Cursor"
	case DialectClaude:
		return "Claude"
	default:
		return "Agent"
	}
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

func IsPost(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	switch n {
	case "posttooluse", "aftershellexecution", "aftermcpexecution", "afterfileedit", "post_tool", "leash.post_tool":
		return true
	default:
		return false
	}
}

func IsFailure(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	switch n {
	case "posttoolusefailure", "tool_error", "leash.tool_error":
		return true
	default:
		return false
	}
}

func IsPlan(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	return n == "plan" || n == "leash.plan"
}

func IsThought(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	return n == "thought" || n == "leash.thought"
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
	return EncodeExtra(ev, d, reason, "")
}

func EncodeExtra(ev Event, d Decision, reason, steer string) []byte {
	if d == DecisionAlways {
		d = DecisionAllow
	}
	reason = mergeSteer(reason, steer, d)
	switch ev.Dialect {
	case DialectCursor:
		return encodeCursor(d, reason, steer)
	case DialectGeneric:
		return encodeGeneric(d, reason, steer)
	default:
		return encodeClaude(ev, d, reason, steer)
	}
}

func SilentAllow(ev Event) []byte {
	return SilentAllowExtra(ev, "")
}

func SilentAllowExtra(ev Event, steer string) []byte {
	if strings.TrimSpace(steer) != "" {
		return EncodeExtra(ev, DecisionAllow, "Allowed by Leash", steer)
	}
	switch ev.Dialect {
	case DialectGeneric:
		return encodeGeneric(DecisionAllow, "Allowed by Leash", "")
	default:
		return []byte("{}\n")
	}
}

func mergeSteer(reason, steer string, d Decision) string {
	steer = strings.TrimSpace(steer)
	if steer == "" {
		return reason
	}
	if d == DecisionKill {
		if reason == "" {
			return steer
		}
		return reason + " — " + steer
	}
	return reason
}

func encodeClaude(ev Event, d Decision, reason, steer string) []byte {
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
	if IsPost(ev) || IsFailure(ev) || IsPlan(ev) || IsThought(ev) {
		spec := map[string]any{"hookEventName": ev.HookEventName}
		if strings.TrimSpace(steer) != "" {
			spec["additionalContext"] = "Operator: " + strings.TrimSpace(steer)
		}
		b, _ := json.Marshal(map[string]any{"hookSpecificOutput": spec})
		return append(b, '\n')
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
	spec := map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       perm,
		"permissionDecisionReason": reason,
	}
	if strings.TrimSpace(steer) != "" && perm == "allow" {
		spec["additionalContext"] = "Operator: " + strings.TrimSpace(steer)
	}
	out := map[string]any{"hookSpecificOutput": spec}
	b, _ := json.Marshal(out)
	return b
}

func encodeCursor(d Decision, reason, steer string) []byte {
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
	msg := reason
	if s := strings.TrimSpace(steer); s != "" && perm == "allow" {
		msg = reason + "\nOperator: " + s
	}
	out := map[string]any{
		"permission":    perm,
		"user_message":  msg,
		"agent_message": msg,
	}
	b, _ := json.Marshal(out)
	return append(b, '\n')
}

func encodeGeneric(d Decision, reason, steer string) []byte {
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
	if s := strings.TrimSpace(steer); s != "" {
		out["steer"] = s
	}
	b, _ := json.Marshal(out)
	return append(b, '\n')
}
