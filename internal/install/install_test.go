package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndUninstallClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := "/opt/leash/leash"
	if err := Install(bin, 540); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/opt/leash/leash") {
		t.Fatalf("missing hook: %s", data)
	}
	codex, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codex), "/opt/leash/leash") {
		t.Fatalf("missing codex hook: %s", codex)
	}
	cfg, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "codex_hooks = true") {
		t.Fatalf("features not enabled: %s", cfg)
	}
	cursor, err := os.ReadFile(filepath.Join(home, ".cursor", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cursor), "/opt/leash/leash") || !strings.Contains(string(cursor), "preToolUse") {
		t.Fatalf("missing cursor hook: %s", cursor)
	}
	if !strings.Contains(string(cursor), "postToolUse") {
		t.Fatalf("missing cursor post hook: %s", cursor)
	}
	plugin, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "plugins", "leash.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plugin), "/opt/leash/leash") || !strings.Contains(string(plugin), "leash-plugin") {
		t.Fatalf("missing opencode plugin: %s", plugin)
	}
	if !strings.Contains(string(plugin), `agent: "OpenCode"`) {
		t.Fatalf("opencode plugin should label the agent: %s", plugin)
	}
	if !strings.Contains(string(plugin), "tool.execute.after") {
		t.Fatalf("opencode plugin should report results: %s", plugin)
	}
	first := strings.Count(string(data), "/opt/leash/leash")
	if first < 1 {
		t.Fatal("missing claude hook")
	}
	if !strings.Contains(string(data), "PostToolUse") {
		t.Fatalf("missing claude post hook: %s", data)
	}
	if err := Install(bin, 540); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if strings.Count(string(data2), "/opt/leash/leash") != first {
		t.Fatalf("hook duplicated: %s", data2)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	data3, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if strings.Contains(string(data3), "/opt/leash/leash") {
		t.Fatalf("still present: %s", data3)
	}
	cursor2, _ := os.ReadFile(filepath.Join(home, ".cursor", "hooks.json"))
	if strings.Contains(string(cursor2), "/opt/leash/leash") {
		t.Fatalf("cursor hook still present: %s", cursor2)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode", "plugins", "leash.js")); !os.IsNotExist(err) {
		t.Fatal("opencode plugin should be removed")
	}
}

func TestInstallQuotesShellButNotOpenCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := "/opt/leash bin/leash"
	if err := Install(bin, 540); err != nil {
		t.Fatal(err)
	}
	claude, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(claude, &root); err != nil {
		t.Fatal(err)
	}
	hooks := root["hooks"].(map[string]any)
	groups := hooks["PreToolUse"].([]any)
	entry := groups[0].(map[string]any)
	cmds := entry["hooks"].([]any)
	cmd := cmds[0].(map[string]any)["command"].(string)
	if cmd != `env LEASH_AGENT=Claude "/opt/leash bin/leash" hook` {
		t.Fatalf("shell hook should quote the binary, got %q", cmd)
	}
	plugin, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "plugins", "leash.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plugin), `const LEASH = "/opt/leash bin/leash"`) {
		t.Fatalf("opencode plugin should pass the raw path to spawnSync: %s", plugin)
	}
	if strings.Contains(string(plugin), `\"/opt/leash bin/leash\"`) {
		t.Fatalf("opencode plugin must not shell-quote the binary: %s", plugin)
	}
}
