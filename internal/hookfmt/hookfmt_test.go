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

func TestParseCursorShell(t *testing.T) {
	raw := []byte(`{
		"conversation_id": "c1",
		"hook_event_name": "beforeShellExecution",
		"command": "rm -rf ./dist",
		"cwd": "/proj"
	}`)
	ev, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Dialect != DialectCursor {
		t.Fatalf("dialect %s", ev.Dialect)
	}
	if ev.ToolName != "Bash" || ev.ToolInput["command"] != "rm -rf ./dist" {
		t.Fatalf("%+v %#v", ev, ev.ToolInput)
	}
}

func TestParseCursorPreTool(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "preToolUse",
		"tool_name": "Shell",
		"tool_input": {"command": "git push -f", "working_directory": "/proj"}
	}`)
	ev, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Dialect != DialectCursor || ev.CWD != "/proj" {
		t.Fatalf("%+v", ev)
	}
}

func TestEncodeCursorDeny(t *testing.T) {
	ev := Event{HookEventName: "preToolUse", Dialect: DialectCursor}
	var out map[string]any
	if err := json.Unmarshal(Encode(ev, DecisionKill, "nope"), &out); err != nil {
		t.Fatal(err)
	}
	if out["permission"] != "deny" {
		t.Fatalf("%v", out)
	}
}

func TestEncodeGeneric(t *testing.T) {
	ev, err := Parse([]byte(`{"protocol":"leash","hook_event_name":"pre_tool","cwd":"/p","tool_name":"bash","tool_input":{"command":"rm -rf x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Dialect != DialectGeneric {
		t.Fatalf("dialect %s", ev.Dialect)
	}
	var out map[string]any
	if err := json.Unmarshal(Encode(ev, DecisionKill, "blocked"), &out); err != nil {
		t.Fatal(err)
	}
	if out["decision"] != "deny" {
		t.Fatalf("%v", out)
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
