package install

import (
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
	if !strings.Contains(string(data), "leash hook") {
		t.Fatalf("missing hook: %s", data)
	}
	codex, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codex), "leash hook") {
		t.Fatalf("missing codex hook: %s", codex)
	}
	cfg, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "codex_hooks = true") {
		t.Fatalf("features not enabled: %s", cfg)
	}
	if err := Install(bin, 540); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if strings.Count(string(data2), "leash hook") != 1 {
		t.Fatalf("hook duplicated: %s", data2)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	data3, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if strings.Contains(string(data3), "leash hook") {
		t.Fatalf("still present: %s", data3)
	}
}
