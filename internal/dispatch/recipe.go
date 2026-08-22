package dispatch

import (
	"fmt"
	"strings"

	"github.com/leashapp/leash/internal/agents"
)

var pickOrder = []string{"opencode", "claude", "cursor-cli", "codex", "hermes", "grok"}

// Recipe is how to run one prompt on an installed CLI agent.
type Recipe struct {
	Name    string
	Command string
	Args    []string
	ACP     bool
}

// Pick chooses an installed CLI agent. Cursor.app is skipped; it has no headless prompt.
func Pick(p agents.Probe, want string) (agents.Found, error) {
	list := agents.Scan(p)
	if want != "" {
		w := strings.ToLower(strings.TrimSpace(want))
		for _, f := range list {
			if !f.Installed || f.ID == "cursor" {
				continue
			}
			if strings.ToLower(f.ID) == w || strings.ToLower(f.Name) == w {
				return f, nil
			}
		}
		return agents.Found{}, fmt.Errorf("no CLI agent named %q", want)
	}
	for _, id := range pickOrder {
		for _, f := range list {
			if f.Installed && f.ID == id {
				return f, nil
			}
		}
	}
	return agents.Found{}, fmt.Errorf("no CLI agent installed — need OpenCode, Claude, Cursor CLI, Codex, Hermes, or Grok")
}

// For returns the launch recipe for a found agent.
func For(f agents.Found, prompt string) (Recipe, error) {
	if f.Path == "" {
		return Recipe{}, fmt.Errorf("%s is not installed", f.Name)
	}
	r := Recipe{Name: f.Name, Command: f.Path}
	switch f.ID {
	case "claude":
		r.Args = []string{"-p", prompt, "--output-format", "text", "--permission-mode", "bypassPermissions"}
	case "opencode":
		r.Args = []string{"run", prompt}
	case "codex":
		r.Args = []string{"exec", "--full-auto", prompt}
	case "cursor-cli":
		r.Args = []string{"-p", prompt, "--print"}
	case "hermes":
		r.ACP = true
		r.Args = []string{"acp"}
	case "grok":
		r.ACP = true
		r.Args = []string{"agent", "stdio"}
	default:
		return Recipe{}, fmt.Errorf("%s cannot run a prompt headless", f.Name)
	}
	return r, nil
}
