package policy

import (
	"path/filepath"
	"strings"
)

// Quiet reports inspection that should not appear on the job tape.
func Quiet(tool, summary string) bool {
	switch normalizeTool(tool) {
	case "Read", "Glob", "Grep", "LS", "WebSearch", "WebFetch", "TodoRead", "TodoWrite":
		if secretPath([]string{summary}) {
			return false
		}
		return true
	case "Bash":
		return !bashMutating(summary)
	default:
		return false
	}
}

// OutcomeFor is the operator-facing consequence of an assessment.
func OutcomeFor(a Assessment) string {
	short := shortTarget(a.Detail, a.Paths)
	tool := firstTool(a.Pattern)
	if s := reasonOutcome(a.Reasons, tool, short); s != "" {
		return s
	}
	return Outcome(tool, a.Detail, a.Paths)
}

// Outcome turns a tool call into a short consequence, not a tool name.
func Outcome(tool, summary string, paths []string) string {
	tool = normalizeTool(tool)
	short := shortTarget(summary, paths)
	switch tool {
	case "Write", "Delete":
		if short != "" {
			return "Write " + short
		}
		return "Write a file"
	case "Edit", "MultiEdit", "NotebookEdit":
		if short != "" {
			return "Change " + short
		}
		return "Change a file"
	case "Read":
		if short != "" {
			return "Look at " + short
		}
		return "Look at a file"
	case "Bash":
		return commandOutcome(summary, short)
	default:
		if short != "" {
			return strings.TrimSpace(toolLabel(tool) + " " + short)
		}
		return toolLabel(tool)
	}
}

func reasonOutcome(reasons []string, tool, short string) string {
	has := func(needle string) bool {
		for _, r := range reasons {
			if r == needle {
				return true
			}
		}
		return false
	}
	switch {
	case has("rm -rf"):
		if short != "" {
			return "Delete " + short
		}
		return "Delete files"
	case has("git force push"):
		return "Force-push"
	case has("git reset --hard"):
		return "Hard-reset"
	case has("git clean -f"):
		return "git clean"
	case has("sudo"):
		if short != "" {
			return "Run with sudo · " + short
		}
		return "Run with sudo"
	case has("download piped to a shell"):
		return "Pipe a download to a shell"
	case has("chmod 777"):
		return "chmod 777"
	case has("drop / reset a database"):
		return "Reset a database"
	case has("kill processes"):
		return "Kill processes"
	case has("find -delete/-exec"):
		return "Find and delete"
	case has("xargs rm"):
		return "xargs rm"
	case has("dd to a device"):
		return "Write to a disk device"
	case has("format a disk"):
		return "Format a disk"
	case has("eval / decode pipeline"):
		return "Eval a decoded pipeline"
	case has("this file looks like a secret"):
		verb := "Touch"
		if normalizeTool(tool) == "Read" {
			verb = "Read"
		}
		if short != "" {
			return verb + " " + short
		}
		return verb + " a secret"
	case has("path is outside the watched folder"):
		if short != "" {
			return "Write outside · " + short
		}
		return "Write outside the project"
	default:
		return ""
	}
}

func commandOutcome(cmd, short string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "Run a command"
	}
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "Run a command"
	}
	switch fields[0] {
	case "rm":
		if short != "" {
			return "Delete " + short
		}
		return "Delete files"
	case "git":
		return gitOutcome(fields, cmd)
	case "npm", "pnpm", "yarn", "bun":
		return "Run " + clipCmd(cmd, 40)
	case "go":
		return "Run " + clipCmd(cmd, 40)
	case "make", "cargo", "python", "python3", "node":
		return "Run " + clipCmd(cmd, 40)
	default:
		if short != "" && looksLikePath(short) {
			return "Run " + clipCmd(fields[0], 24) + " · " + short
		}
		return "Run " + clipCmd(cmd, 48)
	}
}

func gitOutcome(fields []string, cmd string) string {
	if len(fields) < 2 {
		return "Run git"
	}
	switch fields[1] {
	case "status":
		return "Check git status"
	case "diff", "log", "show":
		return "Check git " + fields[1]
	case "push":
		return "git push"
	case "pull", "fetch":
		return "git " + fields[1]
	case "commit":
		return "git commit"
	case "checkout", "switch":
		return "git " + fields[1]
	case "stash":
		return "git stash"
	default:
		return "Run " + clipCmd(cmd, 40)
	}
}

func shortTarget(summary string, paths []string) string {
	for _, p := range paths {
		if s := compactPath(p); s != "" {
			return s
		}
	}
	for _, f := range strings.Fields(summary) {
		f = strings.Trim(f, "\"'")
		if looksLikePath(f) {
			if s := compactPath(f); s != "" {
				return s
			}
		}
	}
	return ""
}

func compactPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	base := filepath.Base(p)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return p
	}
	dir := filepath.Base(filepath.Dir(p))
	switch base {
	case "index.ts", "index.tsx", "index.js", "main.go", "main.ts", "mod.rs":
		if dir != "" && dir != "." && dir != string(filepath.Separator) {
			return dir + "/" + base
		}
	}
	return base
}

func looksLikePath(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return strings.ContainsAny(s, "/\\.") || strings.HasPrefix(s, ".")
}

func clipCmd(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func firstTool(pattern string) string {
	tool, _, ok := strings.Cut(pattern, ":")
	if !ok {
		return ""
	}
	return tool
}

// RemoveRule drops the first always-allow rule that matches tool, pattern, and root.
func RemoveRule(rules []Rule, tool, pattern, root string) []Rule {
	tool = normalizeTool(tool)
	pattern = strings.TrimSpace(pattern)
	out := make([]Rule, 0, len(rules))
	removed := false
	for _, r := range rules {
		if !removed && normalizeTool(r.Tool) == tool && r.Pattern == pattern && sameRuleRoot(r.Root, root) {
			removed = true
			continue
		}
		out = append(out, r)
	}
	return out
}

func sameRuleRoot(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" && b == "" {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	return SameRoot(a, b)
}
