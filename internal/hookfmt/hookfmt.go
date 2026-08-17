package hookfmt

import (
	"encoding/json"
	"strings"
)

// Event is the subset of Claude Code / Codex hook stdin we care about.
type Event struct {
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
}

func Parse(data []byte) (Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Event{}, err
	}
	ev := Event{
		SessionID:     str(raw["session_id"]),
		CWD:           str(raw["cwd"]),
		HookEventName: str(raw["hook_event_name"]),
		ToolName:      str(raw["tool_name"]),
	}
	if ev.HookEventName == "" {
		ev.HookEventName = str(raw["hookEventName"])
	}
	if ev.CWD == "" {
		ev.CWD = str(raw["cwd_path"])
	}
	switch v := raw["tool_input"].(type) {
	case map[string]any:
		ev.ToolInput = v
	default:
		ev.ToolInput = map[string]any{}
	}
	if ev.ToolName == "" {
		ev.ToolName = str(raw["tool"])
	}
	return ev, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func IsPre(ev Event) bool {
	n := strings.ToLower(ev.HookEventName)
	return n == "pretooluse" || n == "permissionrequest" || n == ""
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

// Encode writes the JSON the calling CLI expects on stdout.
func Encode(ev Event, d Decision, reason string) []byte {
	if d == DecisionAlways {
		d = DecisionAllow
	}
	if IsPermissionRequest(ev) {
		behavior := "allow"
		out := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName": "PermissionRequest",
				"decision": map[string]any{
					"behavior": behavior,
				},
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

// SilentAllow is empty stdout so the CLI uses its normal permission flow.
func SilentAllow() []byte {
	return []byte("{}\n")
}
