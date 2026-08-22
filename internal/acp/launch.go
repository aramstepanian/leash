package acp

import (
	"fmt"
	"os"
	"strings"

	"github.com/leashapp/leash/internal/agents"
)

var aliases = map[string][]string{
	"cursor":   {"cursor-agent", "acp"},
	"opencode": {"opencode", "acp"},
	"hermes":   {"hermes", "acp"},
	"grok":     {"grok", "agent", "stdio"},
}

// Launch is the agent command the proxy spawns.
type Launch struct {
	Name    string
	Command string
	Args    []string
}

// ResolveLaunch maps `leash acp cursor` / `leash acp -- cursor-agent acp` onto a binary.
func ResolveLaunch(args []string, p agents.Probe) (Launch, error) {
	args = stripDashDash(args)
	if len(args) == 0 {
		return Launch{}, fmt.Errorf("%s", usageACP(p))
	}
	if extra, ok := aliases[strings.ToLower(args[0])]; ok && len(args) == 1 {
		args = extra
	}
	cmd := args[0]
	rest := args[1:]
	path := cmd
	if !strings.Contains(cmd, string(os.PathSeparator)) {
		if found := agents.FindBin(cmd, p); found != "" {
			path = found
		}
	}
	return Launch{Name: agentLabel(cmd), Command: path, Args: rest}, nil
}

func stripDashDash(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

func agentLabel(cmd string) string {
	base := cmd
	if i := strings.LastIndex(cmd, "/"); i >= 0 {
		base = cmd[i+1:]
	}
	switch base {
	case "cursor-agent":
		return "Cursor"
	case "opencode":
		return "OpenCode"
	case "hermes":
		return "Hermes"
	case "grok":
		return "Grok"
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	default:
		return base
	}
}

func usageACP(p agents.Probe) string {
	b := strings.Builder{}
	b.WriteString("usage: leash acp [--] <agent> [args...]\n")
	b.WriteString("  leash acp cursor\n")
	b.WriteString("  leash acp -- cursor-agent acp\n")
	b.WriteString("  leash acp opencode\n")
	b.WriteString("  leash acp hermes\n")
	b.WriteString("  leash acp grok\n")
	var found []string
	for _, a := range agents.Scan(p) {
		if a.Installed && a.ACP != "" {
			found = append(found, a.Name+"  leash acp -- "+a.ACP)
		}
	}
	if len(found) == 0 {
		b.WriteString("no ACP agents found on this machine")
		return b.String()
	}
	b.WriteString("installed:\n")
	for _, line := range found {
		b.WriteString("  " + line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
