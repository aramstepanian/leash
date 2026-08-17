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

func TestStripSecretsFromToolInput(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "preToolUse",
		"tool_name": "Read",
		"tool_input": {
			"file_path": "/proj/.env",
			"content": "SECRET=1",
			"contents": "SECRET=1",
			"new_string": "nope",
			"old_string": "nope",
			"attachments": [{"data": "x"}]
		}
	}`)
	ev, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"content", "contents", "new_string", "old_string", "attachments"} {
		if _, ok := ev.ToolInput[k]; ok {
			t.Fatalf("secret field %s still present: %+v", k, ev.ToolInput)
		}
	}
	if ev.ToolInput["file_path"] != "/proj/.env" {
		t.Fatalf("path stripped: %+v", ev.ToolInput)
	}
}

func TestAgentLabel(t *testing.T) {
	t.Setenv("LEASH_AGENT", "")
	ev, err := Parse([]byte(`{"hook_event_name":"beforeShellExecution","command":"ls","cwd":"/p"}`))
	if err != nil {
		t.Fatal(err)
	}
	if AgentLabel(ev) != "Cursor" {
		t.Fatalf("cursor: %q", AgentLabel(ev))
	}
	ev, err = Parse([]byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/p"}`))
	if err != nil {
		t.Fatal(err)
	}
	if AgentLabel(ev) != "Claude" {
		t.Fatalf("claude: %q", AgentLabel(ev))
	}
	ev, err = Parse([]byte(`{"protocol":"leash","hook_event_name":"pre_tool","agent":"OpenCode","cwd":"/p","tool_name":"bash"}`))
	if err != nil {
		t.Fatal(err)
	}
	if AgentLabel(ev) != "OpenCode" {
		t.Fatalf("opencode: %q %+v", AgentLabel(ev), ev)
	}
	t.Setenv("LEASH_AGENT", "Codex")
	ev, err = Parse([]byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/p"}`))
	if err != nil {
		t.Fatal(err)
	}
	if AgentLabel(ev) != "Codex" {
		t.Fatalf("env agent: %q", AgentLabel(ev))
	}
}
