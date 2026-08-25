package dispatch

import (
	"fmt"
	"strings"

	"github.com/leashapp/leash/internal/agents"
)

// prefer is auto-pick order: CLI print modes first, then ACP hosts.
var prefer = []string{"claude", "codex", "opencode", "cursor-cli", "hermes", "grok"}

// Canonical maps a user agent name onto a census id.
// "cursor" means the spawnable CLI, not the Mac app.
func Canonical(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	switch s {
	case "", "auto":
		return ""
	case "claude", "claude-code", "claude-cli":
		return "claude"
	case "cursor", "cursor-cli", "cursor-agent", "cursor-app":
		return "cursor-cli"
	case "codex", "openai", "gpt":
		return "codex"
	case "opencode", "open-code":
		return "opencode"
	case "hermes":
		return "hermes"
	case "grok":
		return "grok"
	default:
		return s
	}
}

// Runnable returns installed agents Leash can spawn, in auto-pick order.
func Runnable(found []agents.Found) []agents.Found {
	byID := map[string]agents.Found{}
	for _, f := range found {
		if !CanRun(f) {
			continue
		}
		byID[f.ID] = f
	}
	out := make([]agents.Found, 0, len(byID))
	seen := map[string]bool{}
	for _, id := range prefer {
		if f, ok := byID[id]; ok {
			out = append(out, f)
			seen[id] = true
		}
	}
	for _, f := range found {
		if !seen[f.ID] && CanRun(f) {
			out = append(out, f)
		}
	}
	return out
}

// Candidates is the spawn list for one job.
// want empty: every runnable agent (first is auto-pick).
// want set: that agent, then the rest when fallback is true.
func Candidates(want string, fallback bool, found []agents.Found) ([]agents.Found, error) {
	run := Runnable(found)
	id := Canonical(want)
	if id == "" {
		if len(run) == 0 {
			return nil, fmt.Errorf("no spawnable agent on this Mac — install Claude, Codex, OpenCode, or cursor-agent")
		}
		return run, nil
	}
	var match agents.Found
	for _, f := range run {
		if f.ID == id {
			match = f
			break
		}
	}
	if match.ID == "" {
		if !fallback {
			return nil, missingErr(id, found, run)
		}
		if len(run) == 0 {
			return nil, missingErr(id, found, run)
		}
		return run, nil
	}
	if !fallback {
		return []agents.Found{match}, nil
	}
	out := []agents.Found{match}
	for _, f := range run {
		if f.ID != match.ID {
			out = append(out, f)
		}
	}
	return out, nil
}

func missingErr(id string, found, run []agents.Found) error {
	name := id
	for _, f := range found {
		if f.ID == id {
			name = f.Name
			break
		}
	}
	if len(run) == 0 {
		return fmt.Errorf("%s is not installed, and no other spawnable agent was found", name)
	}
	var names []string
	for _, f := range run {
		names = append(names, f.Name)
	}
	return fmt.Errorf("%s is not installed (try %s, or pass --fallback)", name, strings.Join(names, ", "))
}
