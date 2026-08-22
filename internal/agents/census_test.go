package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsBinAndHook(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "claude"))
	writeExe(t, filepath.Join(bin, "cursor-agent"))
	writeExe(t, filepath.Join(bin, "hermes"))

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{
		"hooks": {"PreToolUse": [{"hooks": [{"command": "/opt/leash/leash hook"}]}]}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	found := Scan(Probe{Home: home, Path: bin})
	got := map[string]Found{}
	for _, f := range found {
		got[f.ID] = f
	}
	if !got["claude"].Installed || !got["claude"].Hooked {
		t.Fatalf("claude: %+v", got["claude"])
	}
	if got["claude"].Door != DoorHooks {
		t.Fatalf("claude door %s", got["claude"].Door)
	}
	if !got["cursor-cli"].Installed || got["cursor-cli"].ACP != "cursor-agent acp" {
		t.Fatalf("cursor-cli: %+v", got["cursor-cli"])
	}
	if !got["hermes"].Installed || got["hermes"].Door != DoorACP {
		t.Fatalf("hermes: %+v", got["hermes"])
	}
	if got["codex"].Installed {
		t.Fatalf("codex should be missing: %+v", got["codex"])
	}
	if got["cursor"].Installed {
		t.Fatalf("cursor app should be missing: %+v", got["cursor"])
	}
}

func TestWellKnownClaudeDir(t *testing.T) {
	home := t.TempDir()
	local := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(local, "claude"))
	p := FindBin("claude", Probe{Home: home, Path: "/nonexistent"})
	if p == "" {
		t.Fatal("expected well-known claude")
	}
}

func TestOpenCodePluginCountsAsHooked(t *testing.T) {
	home := t.TempDir()
	plug := filepath.Join(home, ".config", "opencode", "plugins")
	if err := os.MkdirAll(plug, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plug, "leash.js"), []byte("export const name = 'leash'"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "opencode"))
	found := Scan(Probe{Home: home, Path: bin})
	for _, f := range found {
		if f.ID == "opencode" {
			if !f.Installed || !f.Hooked || f.Door != DoorBoth {
				t.Fatalf("%+v", f)
			}
			return
		}
	}
	t.Fatal("missing opencode")
}

func writeExe(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
