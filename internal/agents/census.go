package agents

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DoorHooks = "hooks"
	DoorACP   = "acp"
	DoorBoth  = "both"
)

// Found is one coding agent Leash can sit in front of.
type Found struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Hooked    bool   `json:"hooked"`
	Door      string `json:"door"`
	Path      string `json:"path,omitempty"`
	ACP       string `json:"acp,omitempty"`
}

// Probe is the host view used to find binaries and hook files.
type Probe struct {
	Home string
	Path string
}

func DefaultProbe() Probe {
	home, _ := os.UserHomeDir()
	if h := os.Getenv("HOME"); h != "" {
		home = h
	}
	return Probe{Home: home, Path: os.Getenv("PATH")}
}

type spec struct {
	id, name, bin, door, acp string
	hook                     func(Probe) string
}

func catalog() []spec {
	return []spec{
		{id: "cursor", name: "Cursor", door: DoorHooks, hook: cursorHooksFile},
		{id: "cursor-cli", name: "Cursor CLI", bin: "cursor-agent", door: DoorACP, acp: "cursor-agent acp"},
		{id: "claude", name: "Claude", bin: "claude", door: DoorHooks, hook: claudeSettingsFile},
		{id: "codex", name: "Codex", bin: "codex", door: DoorHooks, hook: codexHooksFile},
		{id: "opencode", name: "OpenCode", bin: "opencode", door: DoorBoth, acp: "opencode acp", hook: openCodePluginFile},
		{id: "hermes", name: "Hermes", bin: "hermes", door: DoorACP, acp: "hermes acp"},
		{id: "grok", name: "Grok", bin: "grok", door: DoorACP, acp: "grok agent stdio"},
	}
}

// Scan lists known agents and whether this machine has them (and Leash hooks).
func Scan(p Probe) []Found {
	if p.Home == "" {
		p = DefaultProbe()
	}
	out := make([]Found, 0, 7)
	for _, s := range catalog() {
		f := Found{ID: s.id, Name: s.name, Door: s.door, ACP: s.acp}
		switch s.id {
		case "cursor":
			f.Path = cursorApp(p)
			f.Installed = f.Path != ""
		case "cursor-cli":
			f.Path = FindCursorCLI(p)
			f.Installed = f.Path != ""
		default:
			f.Path = FindBin(s.bin, p)
			f.Installed = f.Path != ""
		}
		if s.hook != nil {
			path := s.hook(p)
			if filepath.Base(path) == "leash.js" {
				f.Hooked = fileExists(path)
			} else {
				f.Hooked = fileHasLeash(path)
			}
		}
		out = append(out, f)
	}
	return out
}

// Installed returns agents that exist on this machine.
func Installed(p Probe) []Found {
	var out []Found
	for _, f := range Scan(p) {
		if f.Installed {
			out = append(out, f)
		}
	}
	return out
}

// FindBin locates an executable on PATH, then well-known install dirs.
func FindBin(name string, p Probe) string {
	if name == "" {
		return ""
	}
	sep := string(os.PathListSeparator)
	for _, dir := range strings.Split(p.Path, sep) {
		if dir == "" {
			continue
		}
		cand := filepath.Join(dir, name)
		if isExe(cand) {
			return cand
		}
	}
	for _, cand := range wellKnown(name, p.Home) {
		if isExe(cand) {
			return cand
		}
	}
	return ""
}

func wellKnown(name, home string) []string {
	var out []string
	if home != "" {
		out = append(out,
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, ".cursor", "bin", name),
			filepath.Join(home, "."+name, "local", name),
			filepath.Join(home, ".claude", "local", name),
		)
	}
	out = append(out,
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/usr/local/bin", name),
	)
	return out
}

// FindCursorCLI locates the headless Cursor agent (cursor-agent or agent), including
// installs that live next to Cursor.app rather than on PATH.
func FindCursorCLI(p Probe) string {
	if p.Home == "" {
		p = DefaultProbe()
	}
	for _, name := range []string{"cursor-agent", "agent"} {
		if found := FindBin(name, p); found != "" {
			return found
		}
	}
	if p.Home != "" {
		matches, _ := filepath.Glob(filepath.Join(p.Home, ".local", "share", "cursor-agent", "versions", "*", "cursor-agent"))
		if latest := newestExe(matches); latest != "" {
			return latest
		}
		matches, _ = filepath.Glob(filepath.Join(p.Home, ".local", "share", "cursor-agent", "versions", "*", "agent"))
		if latest := newestExe(matches); latest != "" {
			return latest
		}
	}
	app := cursorApp(p)
	if app != "" {
		for _, rel := range []string{
			"Contents/Resources/app/bin/cursor-agent",
			"Contents/Resources/app/bin/agent",
			"Contents/Resources/cursor-agent",
			"Contents/MacOS/cursor-agent",
		} {
			cand := filepath.Join(app, rel)
			if isExe(cand) {
				return cand
			}
		}
	}
	return ""
}

func newestExe(paths []string) string {
	var best string
	var bestMod int64
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		mod := info.ModTime().UnixNano()
		if best == "" || mod >= bestMod {
			best, bestMod = p, mod
		}
	}
	return best
}

func cursorApp(p Probe) string {
	var cands []string
	if runtime.GOOS == "darwin" {
		cands = append(cands, "/Applications/Cursor.app", filepath.Join(p.Home, "Applications", "Cursor.app"))
	}
	for _, c := range cands {
		if isDir(c) {
			return c
		}
	}
	// A hooks file alone is not proof the app is installed.
	return ""
}

func cursorHooksFile(p Probe) string {
	return filepath.Join(p.Home, ".cursor", "hooks.json")
}

func claudeSettingsFile(p Probe) string {
	return filepath.Join(p.Home, ".claude", "settings.json")
}

func codexHooksFile(p Probe) string {
	return filepath.Join(p.Home, ".codex", "hooks.json")
}

func openCodePluginFile(p Probe) string {
	return filepath.Join(p.Home, ".config", "opencode", "plugins", "leash.js")
}

func fileHasLeash(path string) bool {
	if path == "" {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return false
	}
	s := string(data)
	if strings.Contains(s, "leash hook") || strings.Contains(s, "leash.js") {
		return true
	}
	if strings.Contains(strings.ToLower(s), "leash") && strings.Contains(s, "hook") {
		return true
	}
	var raw any
	if json.Unmarshal(data, &raw) == nil {
		return jsonHasLeash(raw)
	}
	return false
}

func jsonHasLeash(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.Contains(t, "leash hook") || (strings.Contains(strings.ToLower(t), "leash") && strings.HasSuffix(strings.TrimSpace(t), "hook"))
	case []any:
		for _, x := range t {
			if jsonHasLeash(x) {
				return true
			}
		}
	case map[string]any:
		for _, x := range t {
			if jsonHasLeash(x) {
				return true
			}
		}
	}
	return false
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func isExe(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
