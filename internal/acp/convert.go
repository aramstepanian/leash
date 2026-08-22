package acp

import (
	"encoding/json"
	"strings"

	"github.com/leashapp/leash/internal/hookfmt"
)

type permOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type permParams struct {
	SessionID string          `json:"sessionId"`
	ToolCall  json.RawMessage `json:"toolCall"`
	Options   []permOption    `json:"options"`
}

type location struct {
	Path string `json:"path"`
}

type toolCall struct {
	ToolCallID string          `json:"toolCallId"`
	Title      string          `json:"title"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Locations  []location      `json:"locations"`
	RawInput   json.RawMessage `json:"rawInput"`
}

type sessionNew struct {
	Cwd string `json:"cwd"`
}

type sessionUpdate struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

type updateBody struct {
	SessionUpdate string          `json:"sessionUpdate"`
	Title         string          `json:"title"`
	Kind          string          `json:"kind"`
	Status        string          `json:"status"`
	Entries       []planEntry     `json:"entries"`
	Content       json.RawMessage `json:"content"`
	Locations     []location      `json:"locations"`
	RawInput      json.RawMessage `json:"rawInput"`
	ToolCallID    string          `json:"toolCallId"`
}

type planEntry struct {
	Content string `json:"content"`
	Status  string `json:"status"`
}

// EventFromPermission turns an ACP permission request into a Leash pre-tool event.
func EventFromPermission(agent, cwd string, raw json.RawMessage) hookfmt.Event {
	var p permParams
	_ = json.Unmarshal(raw, &p)
	return eventFromToolCall(agent, cwd, "pre_tool", p.ToolCall)
}

func eventFromToolCall(agent, cwd, hook string, raw json.RawMessage) hookfmt.Event {
	var tc toolCall
	_ = json.Unmarshal(raw, &tc)
	input := map[string]any{}
	if len(tc.RawInput) > 0 && json.Unmarshal(tc.RawInput, &input) != nil {
		input = map[string]any{}
	}
	if input == nil {
		input = map[string]any{}
	}
	for _, loc := range tc.Locations {
		if loc.Path == "" {
			continue
		}
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = loc.Path
		}
	}
	tool, command := mapKind(tc.Kind, tc.Title, input)
	if command != "" {
		if _, ok := input["command"]; !ok {
			input["command"] = command
		}
	}
	if cwd == "" {
		if s, _ := input["cwd"].(string); s != "" {
			cwd = s
		}
	}
	return hookfmt.Event{
		CWD:           cwd,
		HookEventName: hook,
		ToolName:      tool,
		ToolInput:     input,
		Dialect:       hookfmt.DialectGeneric,
		Agent:         agent,
		Text:          tc.Title,
	}
}

func mapKind(kind, title string, input map[string]any) (tool, command string) {
	if cmd, _ := input["command"].(string); cmd != "" {
		return "Bash", cmd
	}
	switch strings.ToLower(kind) {
	case "execute":
		return "Bash", firstNonEmpty(str(input["cmd"]), title)
	case "edit", "delete", "move":
		return "Edit", ""
	case "read", "search":
		return "Read", ""
	case "fetch":
		return "WebFetch", ""
	default:
		if looksLikeShell(title) {
			return "Bash", title
		}
		if title != "" {
			return title, ""
		}
		return "tool", ""
	}
}

func looksLikeShell(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "|&;<>") {
		return true
	}
	head, _, _ := strings.Cut(s, " ")
	switch head {
	case "rm", "git", "npm", "pnpm", "yarn", "go", "cargo", "make", "sudo", "curl", "chmod", "find", "xargs":
		return true
	}
	return false
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// PickOption maps a Leash decision onto an ACP permission option id.
func PickOption(raw json.RawMessage, d hookfmt.Decision) (optionID string, cancelled bool) {
	var p permParams
	_ = json.Unmarshal(raw, &p)
	want := []string{"allow_once"}
	if d == hookfmt.DecisionAlways {
		want = []string{"allow_always", "allow_once"}
	}
	if d == hookfmt.DecisionKill {
		want = []string{"reject_once", "reject_always"}
	}
	for _, kind := range want {
		for _, opt := range p.Options {
			if opt.Kind == kind {
				return opt.OptionID, false
			}
		}
	}
	if d == hookfmt.DecisionKill {
		return "", true
	}
	for _, opt := range p.Options {
		if strings.HasPrefix(opt.Kind, "allow") {
			return opt.OptionID, false
		}
	}
	return "", true
}

func permissionResult(optionID string, cancelled bool) []byte {
	var out any
	if cancelled || optionID == "" {
		out = map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}
	} else {
		out = map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": optionID}}
	}
	b, _ := json.Marshal(out)
	return b
}

func eventFromUpdate(agent, cwd string, raw json.RawMessage) (hookfmt.Event, bool) {
	var su sessionUpdate
	if json.Unmarshal(raw, &su) != nil {
		return hookfmt.Event{}, false
	}
	var u updateBody
	if json.Unmarshal(su.Update, &u) != nil {
		return hookfmt.Event{}, false
	}
	switch u.SessionUpdate {
	case "plan":
		var steps []string
		var title string
		for _, e := range u.Entries {
			if e.Content == "" {
				continue
			}
			steps = append(steps, e.Content)
			if title == "" {
				title = e.Content
			}
		}
		return hookfmt.Event{
			CWD: cwd, HookEventName: "plan", Dialect: hookfmt.DialectGeneric,
			Agent: agent, Text: title, Steps: steps,
		}, title != ""
	case "tool_call", "tool_call_update":
		if u.Status != "completed" && u.Status != "failed" {
			return hookfmt.Event{}, false
		}
		rawTC, _ := json.Marshal(u)
		hook := "post_tool"
		if u.Status == "failed" {
			hook = "leash.tool_error"
		}
		ev := eventFromToolCall(agent, cwd, hook, rawTC)
		if u.Status == "failed" {
			ev.Text = firstNonEmpty(u.Title, "tool failed")
		}
		return ev, true
	default:
		return hookfmt.Event{}, false
	}
}

func cwdFromParams(raw json.RawMessage) string {
	var p sessionNew
	_ = json.Unmarshal(raw, &p)
	return strings.TrimSpace(p.Cwd)
}
