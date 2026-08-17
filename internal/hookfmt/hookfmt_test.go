package hookfmt

import (
	"encoding/json"
	"testing"
)

func TestParseClaudeBash(t *testing.T) {
	raw := []byte(`{
		"session_id": "abc",
		"cwd": "/proj",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf /tmp"}
	}`)
	ev, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ToolName != "Bash" || ev.CWD != "/proj" {
		t.Fatalf("%+v", ev)
	}
	if ev.ToolInput["command"] != "rm -rf /tmp" {
		t.Fatalf("input %+v", ev.ToolInput)
	}
}

func TestEncodeDeny(t *testing.T) {
	ev := Event{HookEventName: "PreToolUse"}
	var out map[string]any
	if err := json.Unmarshal(Encode(ev, DecisionKill, "nope"), &out); err != nil {
		t.Fatal(err)
	}
	h := out["hookSpecificOutput"].(map[string]any)
	if h["permissionDecision"] != "deny" {
		t.Fatalf("%v", h)
	}
}

func TestEncodeCodexPermission(t *testing.T) {
	ev := Event{HookEventName: "PermissionRequest"}
	var out map[string]any
	if err := json.Unmarshal(Encode(ev, DecisionKill, "blocked"), &out); err != nil {
		t.Fatal(err)
	}
	h := out["hookSpecificOutput"].(map[string]any)
	d := h["decision"].(map[string]any)
	if d["behavior"] != "deny" {
		t.Fatalf("%v", d)
	}
}
