package acp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/server"
)

// DaemonGate posts a permission through the local Leash daemon.
// If the daemon is down, it allows (fail open, same as hooks).
func DaemonGate(port int, token string) Gate {
	return func(ctx context.Context, ev hookfmt.Event) hookfmt.Decision {
		if token == "" || port == 0 || !server.DaemonRunning(port) {
			return hookfmt.DecisionAllow
		}
		body, err := json.Marshal(map[string]any{
			"protocol":        "leash",
			"hook_event_name": "pre_tool",
			"agent":           ev.Agent,
			"cwd":             ev.CWD,
			"tool_name":       ev.ToolName,
			"tool_input":      ev.ToolInput,
			"text":            ev.Text,
		})
		if err != nil {
			return hookfmt.DecisionAllow
		}
		timeout := 9 * time.Minute
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
		}
		out, code, err := server.PostHook(port, token, body, timeout)
		if err != nil || code != 200 {
			return hookfmt.DecisionAllow
		}
		var parsed struct {
			Decision string `json:"decision"`
		}
		if json.Unmarshal(out, &parsed) != nil {
			return hookfmt.DecisionAllow
		}
		if parsed.Decision == "deny" {
			return hookfmt.DecisionKill
		}
		return hookfmt.DecisionAllow
	}
}

// DaemonNotify sends plan / post-tool events without blocking the ACP stream.
func DaemonNotify(port int, token string) Notify {
	return func(ev hookfmt.Event) {
		if token == "" || port == 0 || !server.DaemonRunning(port) {
			return
		}
		body, err := json.Marshal(map[string]any{
			"protocol":        "leash",
			"hook_event_name": ev.HookEventName,
			"agent":           ev.Agent,
			"cwd":             ev.CWD,
			"tool_name":       ev.ToolName,
			"tool_input":      ev.ToolInput,
			"text":            ev.Text,
			"steps":           ev.Steps,
		})
		if err != nil {
			return
		}
		_, _, _ = server.PostHook(port, token, body, 2*time.Second)
	}
}
