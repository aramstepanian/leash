package dispatch

import (
	"fmt"
	"strings"

	"github.com/leashapp/leash/internal/agents"
)

type Mode string

const (
	ModeCLI Mode = "cli"
	ModeACP Mode = "acp"
)

// Recipe is how Leash starts one installed agent.
type Recipe struct {
	ID      string
	Name    string
	Mode    Mode
	Command string
	Args    []string
}

func CanRun(f agents.Found) bool {
	_, err := RecipeOf(f)
	return err == nil
}

func RecipeOf(f agents.Found) (Recipe, error) {
	if f.ID == "cursor" {
		return Recipe{}, fmt.Errorf("Cursor app is not spawnable — use cursor-agent")
	}
	if !f.Installed || strings.TrimSpace(f.Path) == "" {
		return Recipe{}, fmt.Errorf("%s is not installed", displayName(f))
	}
	base := Recipe{ID: f.ID, Name: displayName(f), Command: f.Path}
	switch f.ID {
	case "claude":
		base.Mode = ModeCLI
		base.Args = []string{"-p"}
		return base, nil
	case "codex":
		base.Mode = ModeCLI
		base.Args = []string{"exec"}
		return base, nil
	case "opencode":
		base.Mode = ModeCLI
		base.Args = []string{"run"}
		return base, nil
	}
	if strings.TrimSpace(f.ACP) == "" {
		return Recipe{}, fmt.Errorf("%s cannot be started by Leash", displayName(f))
	}
	argv := strings.Fields(f.ACP)
	base.Mode = ModeACP
	if len(argv) > 1 {
		base.Args = argv[1:]
	}
	return base, nil
}

// Argv is the process args. CLI recipes append the task as the last argument.
func (r Recipe) Argv(prompt string) []string {
	args := append([]string{}, r.Args...)
	if r.Mode == ModeCLI {
		args = append(args, prompt)
	}
	return args
}

func displayName(f agents.Found) string {
	if strings.TrimSpace(f.Name) != "" {
		return f.Name
	}
	return f.ID
}
