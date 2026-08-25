package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/mission"
	"github.com/leashapp/leash/internal/policy"
)

func (s *Server) handleSteer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		http.Error(w, "text required", 400)
		return
	}
	s.mission.SetSteer(text)
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "steer",
		Title:  "Steer",
		Detail: text,
		Result: "ok",
	})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleInterrupt(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body)
	text := strings.TrimSpace(body.Text)
	if text != "" {
		s.mission.SetSteer(text)
	}
	s.mission.ArmInterrupt()
	s.mu.Lock()
	p := oldestPending(s.pending)
	id := ""
	if p != nil {
		id = p.ID
	}
	s.mu.Unlock()
	stopped := s.StopJob(firstNonEmpty(text, "operator interrupt"))
	if id != "" {
		_ = s.Resolve(id, hookfmt.DecisionKill)
	} else if !stopped {
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "interrupt",
			Title:  "Interrupt",
			Detail: firstNonEmpty(text, "operator interrupt"),
			Result: "deny",
		})
		s.mission.ClearLive()
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleRetry(w http.ResponseWriter, r *http.Request) {
	f := s.mission.Failed()
	if f == nil {
		http.Error(w, "nothing to retry", 400)
		return
	}
	note := "Retry the last failed step"
	if f.Outcome != "" {
		note = "Retry: " + f.Outcome
	} else if f.Tool != "" {
		note = "Retry the last failed tool: " + f.Tool
	}
	if f.Error != "" {
		note += " — " + f.Error
	}
	s.mission.SetSteer(note)
	s.mission.ClearFailed()
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "steer",
		Title:  "Retry",
		Detail: note,
		Tool:   f.Tool,
		Result: "ok",
	})
	writeJSON(w, map[string]any{"ok": true, "steer": note})
}

func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	f := s.mission.Failed()
	if f == nil {
		http.Error(w, "nothing to skip", 400)
		return
	}
	s.mission.ClearFailed()
	s.mission.ClearLive()
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "skip",
		Title:  "Skip",
		Detail: firstNonEmpty(f.Outcome, f.Tool) + " failed",
		Tool:   f.Tool,
		Error:  f.Error,
		Result: "ok",
	})
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) recordPlan(ev hookfmt.Event, root string) {
	title := "Plan"
	detail := ev.Text
	if len(ev.Steps) > 0 {
		detail = strings.Join(ev.Steps, " → ")
		if ev.Text != "" {
			title = ev.Text
		}
	}
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "plan",
		Agent:  hookfmt.AgentLabel(ev),
		Title:  title,
		Detail: detail,
		Root:   root,
		Result: "ok",
	})
}

func (s *Server) recordThought(ev hookfmt.Event, root string) {
	_ = ev
	_ = root
}

func (s *Server) handlePostEvent(ev hookfmt.Event, root string) []byte {
	agent := hookfmt.AgentLabel(ev)
	summary := policy.CommandSummary(ev.ToolName, ev.ToolInput)
	errText := ""
	result := "ok"
	if hookfmt.IsFailure(ev) || looksFailed(ev.Text) {
		result = "error"
		errText = ev.Text
	}
	if result != "error" && policy.Quiet(ev.ToolName, summary) {
		return hookfmt.SilentAllow(ev)
	}
	live := s.mission.FinishLive(result, ev.Text, errText, ev.DurationMs)
	tool, detail := ev.ToolName, summary
	dur := ev.DurationMs
	outcome := ""
	if live != nil {
		if tool == "" {
			tool = live.Tool
		}
		if detail == "" || detail == ev.ToolName {
			detail = live.Detail
		}
		if dur == 0 {
			dur = live.DurationMs
		}
		agent = firstNonEmpty(agent, live.Agent)
		root = firstNonEmpty(root, live.Root)
		outcome = live.Outcome
	}
	kind := "tool"
	if result == "error" {
		kind = "error"
	}
	paths := policy.Paths(ev.ToolName, ev.CWD, ev.ToolInput)
	if outcome == "" {
		outcome = policy.Outcome(tool, detail, paths)
	}
	s.mission.Append(mission.Event{
		ID:         newID(),
		Kind:       kind,
		Agent:      agent,
		Tool:       tool,
		Title:      outcome,
		Detail:     firstNonEmpty(errText, ev.Text, detail),
		Result:     result,
		Error:      errText,
		DurationMs: dur,
		Paths:      paths,
		Root:       root,
	})
	if result == "error" && live == nil {
		s.mission.MarkFailed(mission.Failed{
			Tool: tool, Detail: detail, Outcome: outcome, Error: firstNonEmpty(errText, ev.Text), Agent: agent,
		})
	}
	if result == "ok" && len(paths) > 0 && isWriteTool(ev.ToolName) {
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "diff",
			Agent:  agent,
			Tool:   ev.ToolName,
			Title:  "Changed files",
			Detail: strings.Join(baseNames(paths), " · "),
			Paths:  paths,
			Root:   root,
			Result: "ok",
		})
	}
	return hookfmt.SilentAllow(ev)
}

func looksFailed(text string) bool {
	low := strings.ToLower(text)
	if low == "" {
		return false
	}
	for _, n := range []string{"exit status", "exit code", "error:", "failed", "fatal:", "panic:", "traceback"} {
		if strings.Contains(low, n) {
			return true
		}
	}
	return false
}

func isWriteTool(tool string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "write", "edit", "multiedit", "notebookedit", "str_replace", "strreplace", "apply_patch", "applypatch", "delete", "remove":
		return true
	default:
		return false
	}
}

func baseNames(paths []string) []string {
	var out []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		out = append(out, p)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func visibleTape(events []mission.Event) []mission.Event {
	out := make([]mission.Event, 0, len(events))
	for _, e := range events {
		if tapeQuiet(e) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func tapeQuiet(e mission.Event) bool {
	switch e.Kind {
	case "thought":
		return true
	case "tool":
		if e.Result == "error" {
			return false
		}
		return policy.Quiet(e.Tool, e.Detail)
	default:
		return false
	}
}

func (s *Server) replyPre(ev hookfmt.Event, d hookfmt.Decision, reason string) []byte {
	kill, steer := s.mission.ConsumePre()
	if kill {
		d = hookfmt.DecisionKill
		reason = firstNonEmpty(steer, "Interrupted by operator")
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "interrupt",
			Agent:  hookfmt.AgentLabel(ev),
			Title:  "Interrupt",
			Detail: reason,
			Result: "deny",
		})
		s.mission.ClearLive()
		return hookfmt.EncodeExtra(ev, d, reason, steer)
	}
	if d == hookfmt.DecisionKill {
		return hookfmt.EncodeExtra(ev, d, reason, steer)
	}
	if strings.TrimSpace(steer) == "" {
		return hookfmt.SilentAllow(ev)
	}
	return hookfmt.EncodeExtra(ev, d, reason, steer)
}
