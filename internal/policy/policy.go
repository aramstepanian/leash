package policy

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Verdict is what Leash should do before a tool call runs.
type Verdict int

const (
	// Allow the tool with no UI. Still snapshot if Mutating.
	Allow Verdict = iota
	// Ask shows the native Allow / Always / Kill panel.
	Ask
)

// Assessment is the policy result for one tool call.
type Assessment struct {
	Verdict  Verdict
	Kind     string // "shell", "secret", "outside", "destroy", "write"
	Title    string
	Detail   string
	Reasons  []string
	Mutating bool
	Pattern  string // Always-allow key, e.g. "Bash:npm test"
	Paths    []string
}

type Rule struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern"`
}

var (
	reRmRf       = regexp.MustCompile(`(?i)\brm\s+(-[a-zA-Z]*r[a-zA-Z]*f|-[a-zA-Z]*f[a-zA-Z]*r|--recursive\s+--force|--force\s+--recursive)`)
	rePipeShell  = regexp.MustCompile(`(?i)(curl|wget|fetch)\b[\s\S]{0,200}\|\s*(sudo\s+)?(ba)?sh\b`)
	reForcePush  = regexp.MustCompile(`(?i)\bgit\s+push\b[\s\S]*(\s(-f|--force|--force-with-lease)\b)`)
	reResetHard  = regexp.MustCompile(`(?i)\bgit\s+reset\s+--hard\b`)
	reCleanForce = regexp.MustCompile(`(?i)\bgit\s+clean\s+-[a-zA-Z]*f`)
	reSudo       = regexp.MustCompile(`(?i)(^|[;&]|&&|\|\|)\s*sudo\b`)
	reDD         = regexp.MustCompile(`(?i)\bdd\s+.*\bof=/dev/`)
	reMkfs       = regexp.MustCompile(`(?i)\bmkfs(\.\w+)?\b`)
	reChmod777   = regexp.MustCompile(`(?i)\bchmod\s+(-R\s+)?([0-7]*7[0-7]*7|a\+rwx|777)\b`)
	reDropDB     = regexp.MustCompile(`(?i)\b(drop\s+(table|database)|prisma\s+migrate\s+reset|migrate\s+reset)\b`)
	reCurlExec   = regexp.MustCompile(`(?i)\b(eval|base64\s+-d|base64\s+--decode)\b`)
	reFindDelete = regexp.MustCompile(`(?i)\bfind\b[\s\S]{0,400}\s-(delete|exec)\b`)
	reXargsRm    = regexp.MustCompile(`(?i)\bxargs\b[\s\S]{0,80}\brm\b`)
	reKillAll    = regexp.MustCompile(`(?i)\b(pkill|killall|kill\s+-9)\b`)
)

var secretNames = []string{
	".env", ".env.local", ".env.production", ".env.development",
	".npmrc", ".pypirc", ".netrc", ".git-credentials",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	"credentials.json", "service-account.json",
}

var secretExt = []string{".pem", ".p12", ".pfx", ".key", ".keystore"}

var secretPathBits = []string{
	"/.ssh/", "/.aws/credentials", "/.gnupg/", "/.kube/config",
}

var skipSnap = []string{
	"node_modules", ".git", "dist", "build", ".next", "target",
	"vendor", ".venv", "venv", "__pycache__", ".turbo", "coverage",
}

// Assess decides whether to prompt. watchRoot may be empty.
func Assess(tool, cwd, watchRoot string, input map[string]any, always []Rule) Assessment {
	tool = normalizeTool(tool)
	paths := Paths(tool, cwd, input)
	summary := CommandSummary(tool, input)
	a := Assessment{
		Kind:     "write",
		Title:    toolLabel(tool),
		Detail:   summary,
		Mutating: isMutating(tool, summary, paths),
		Pattern:  alwaysKey(tool, summary),
		Paths:    paths,
	}

	if matchAlways(always, tool, summary, paths) {
		a.Verdict = Allow
		a.Reasons = []string{"always-allow rule"}
		return a
	}

	if tool == "Read" || tool == "Glob" || tool == "Grep" || tool == "LS" || tool == "WebSearch" || tool == "WebFetch" || tool == "TodoRead" || tool == "TodoWrite" {
		if secretPath(paths) {
			a.Verdict = Ask
			a.Kind = "secret"
			a.Title = "Read secret"
			a.Reasons = []string{"this file looks like a secret"}
			a.Mutating = false
			return a
		}
		a.Verdict = Allow
		a.Mutating = false
		return a
	}

	if reasons := dangerousShell(summary); len(reasons) > 0 && isShell(tool) {
		a.Verdict = Ask
		a.Kind = "destroy"
		a.Title = "Dangerous command"
		a.Reasons = reasons
		a.Mutating = true
		return a
	}

	if secretPath(paths) {
		a.Verdict = Ask
		a.Kind = "secret"
		a.Title = "Touch secret"
		a.Reasons = []string{"this file looks like a secret"}
		a.Mutating = true
		return a
	}

	if watchRoot != "" && outsideRoot(watchRoot, paths) {
		a.Verdict = Ask
		a.Kind = "outside"
		a.Title = "Outside project"
		a.Reasons = []string{"path is outside the watched folder"}
		a.Mutating = true
		return a
	}

	if isShell(tool) && looksDestructivePath(summary, paths) {
		a.Verdict = Ask
		a.Kind = "destroy"
		a.Title = "Destructive command"
		a.Reasons = []string{"command looks like it deletes files"}
		a.Mutating = true
		return a
	}

	a.Verdict = Allow
	if isShell(tool) {
		a.Kind = "shell"
	}
	return a
}

func matchAlways(rules []Rule, tool, summary string, paths []string) bool {
	for _, r := range rules {
		if r.Tool != "" && !strings.EqualFold(normalizeTool(r.Tool), tool) {
			continue
		}
		p := strings.TrimSpace(r.Pattern)
		if p == "" {
			continue
		}
		if summary == p {
			return true
		}
		for _, path := range paths {
			if path == p {
				return true
			}
			if !strings.Contains(p, "/") && !strings.Contains(p, string(filepath.Separator)) && filepath.Base(path) == p {
				return true
			}
		}
	}
	return false
}

func alwaysKey(tool, summary string) string {
	s := strings.TrimSpace(summary)
	if len(s) > 80 {
		s = s[:80]
	}
	return tool + ":" + s
}

func normalizeTool(tool string) string {
	t := strings.TrimSpace(tool)
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "mcp:") {
		t = strings.TrimSpace(t[4:])
		low = strings.ToLower(t)
	}
	switch low {
	case "bash", "shell", "bashtool", "powershell":
		return "Bash"
	case "apply_patch", "applypatch", "edit", "multiedit", "notebookedit", "str_replace", "strreplace":
		return "Edit"
	case "write", "delete", "remove":
		return "Write"
	case "read", "readfile", "read_file":
		return "Read"
	default:
		return t
	}
}

func isShell(tool string) bool {
	return normalizeTool(tool) == "Bash"
}

func toolLabel(tool string) string {
	switch normalizeTool(tool) {
	case "Bash":
		return "Run command"
	case "Write":
		return "Write file"
	case "Edit", "MultiEdit", "NotebookEdit":
		return "Edit file"
	case "Read":
		return "Read file"
	default:
		return tool
	}
}

func CommandSummary(tool string, input map[string]any) string {
	if input == nil {
		return tool
	}
	for _, k := range []string{"command", "cmd", "script"} {
		if s, ok := stringVal(input[k]); ok && s != "" {
			return strings.TrimSpace(s)
		}
	}
	if s, ok := stringVal(input["file_path"]); ok && s != "" {
		return s
	}
	if s, ok := stringVal(input["filePath"]); ok && s != "" {
		return s
	}
	if s, ok := stringVal(input["path"]); ok && s != "" {
		return s
	}
	return tool
}

func Paths(tool, cwd string, input map[string]any) []string {
	var out []string
	if input != nil {
		for _, k := range []string{"file_path", "path", "target_file", "notebook_path", "filePath", "targetFile"} {
			if s, ok := stringVal(input[k]); ok && s != "" {
				out = append(out, abs(cwd, s))
			}
		}
	}
	summary := CommandSummary(tool, input)
	if isShell(tool) {
		out = append(out, shellPaths(cwd, summary)...)
	}
	return uniq(out)
}

func stringVal(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func abs(cwd, p string) string {
	if p == "" {
		return p
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if cwd == "" {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(cwd, p))
}

func shellPaths(cwd, cmd string) []string {
	fields := strings.Fields(cmd)
	var out []string
	for _, f := range fields {
		if strings.HasPrefix(f, "-") {
			continue
		}
		if strings.ContainsAny(f, "|&;<>()$`") {
			continue
		}
		if strings.Contains(f, "/") || strings.HasPrefix(f, ".") || strings.Contains(f, ".") && (strings.HasSuffix(f, ".go") || strings.HasSuffix(f, ".ts") || strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".py") || strings.HasSuffix(f, ".rs") || strings.HasSuffix(f, ".env")) {
			out = append(out, abs(cwd, strings.Trim(f, "\"'")))
		}
	}
	return out
}

func dangerousShell(cmd string) []string {
	var reasons []string
	if reRmRf.MatchString(cmd) {
		reasons = append(reasons, "rm -rf")
	}
	if rePipeShell.MatchString(cmd) {
		reasons = append(reasons, "download piped to a shell")
	}
	if reForcePush.MatchString(cmd) {
		reasons = append(reasons, "git force push")
	}
	if reResetHard.MatchString(cmd) {
		reasons = append(reasons, "git reset --hard")
	}
	if reCleanForce.MatchString(cmd) {
		reasons = append(reasons, "git clean -f")
	}
	if reSudo.MatchString(cmd) {
		reasons = append(reasons, "sudo")
	}
	if reDD.MatchString(cmd) {
		reasons = append(reasons, "dd to a device")
	}
	if reMkfs.MatchString(cmd) {
		reasons = append(reasons, "format a disk")
	}
	if reChmod777.MatchString(cmd) {
		reasons = append(reasons, "chmod 777")
	}
	if reDropDB.MatchString(cmd) {
		reasons = append(reasons, "drop / reset a database")
	}
	if reCurlExec.MatchString(cmd) && strings.Contains(cmd, "|") {
		reasons = append(reasons, "eval / decode pipeline")
	}
	if reKillAll.MatchString(cmd) {
		reasons = append(reasons, "kill processes")
	}
	if reFindDelete.MatchString(cmd) {
		reasons = append(reasons, "find -delete/-exec")
	}
	if reXargsRm.MatchString(cmd) {
		reasons = append(reasons, "xargs rm")
	}
	return reasons
}

func looksDestructivePath(cmd string, paths []string) bool {
	low := strings.ToLower(cmd)
	if strings.Contains(low, " rm ") || strings.HasPrefix(low, "rm ") {
		return true
	}
	if strings.Contains(low, "unlink ") || strings.Contains(low, "shred ") {
		return true
	}
	_ = paths
	return false
}

func secretPath(paths []string) bool {
	for _, p := range paths {
		base := strings.ToLower(filepath.Base(p))
		for _, n := range secretNames {
			if base == n || strings.HasPrefix(base, ".env.") {
				return true
			}
		}
		for _, ext := range secretExt {
			if strings.HasSuffix(base, ext) {
				return true
			}
		}
		low := strings.ToLower(filepath.ToSlash(p))
		for _, bit := range secretPathBits {
			if strings.Contains(low, bit) {
				return true
			}
		}
	}
	return false
}

func outsideRoot(root string, paths []string) bool {
	if root == "" || len(paths) == 0 {
		return false
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	for _, p := range paths {
		ap, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(r, ap)
		if err != nil {
			return true
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isMutating(tool, summary string, paths []string) bool {
	switch normalizeTool(tool) {
	case "Read", "Glob", "Grep", "LS", "WebSearch", "WebFetch", "TodoRead", "TodoWrite":
		return false
	case "Bash":
		return bashMutating(summary)
	default:
		return len(paths) > 0 || normalizeTool(tool) == "Write" || normalizeTool(tool) == "Edit"
	}
}

func bashMutating(summary string) bool {
	low := strings.ToLower(strings.TrimSpace(summary))
	if low == "" {
		return false
	}
	if strings.ContainsAny(low, ";&|><") {
		return true
	}
	fields := strings.Fields(low)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "ls", "pwd", "cat", "head", "tail", "rg", "grep", "echo", "which", "whoami", "date", "wc", "true", "false":
		return false
	case "git":
		if len(fields) < 2 {
			return false
		}
		switch fields[1] {
		case "status", "diff", "log", "show", "rev-parse", "describe":
			return false
		case "branch":
			for _, f := range fields[2:] {
				if f == "-d" || f == "-D" || strings.HasPrefix(f, "--delete") {
					return true
				}
			}
			return false
		default:
			return true
		}
	case "find":
		return strings.Contains(low, "-delete") || strings.Contains(low, "-exec")
	case "env":
		return len(fields) > 1
	default:
		return true
	}
}

func SkipSnapshot(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		for _, s := range skipSnap {
			if p == s {
				return true
			}
		}
	}
	return false
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
