package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leashapp/leash/internal/agents"
)

func TestCanonical(t *testing.T) {
	if Canonical("Claude Code") != "claude" {
		t.Fatalf("%q", Canonical("Claude Code"))
	}
	if Canonical("cursor") != "cursor-cli" {
		t.Fatalf("cursor should map to CLI, got %q", Canonical("cursor"))
	}
	if Canonical("auto") != "" {
		t.Fatalf("auto %q", Canonical("auto"))
	}
}

func TestCandidatesAutoOrder(t *testing.T) {
	found := []agents.Found{
		{ID: "cursor", Name: "Cursor", Installed: true, Path: "/Apps/Cursor.app"},
		{ID: "cursor-cli", Name: "Cursor CLI", Installed: true, Path: "/bin/cursor-agent", ACP: "cursor-agent acp"},
		{ID: "claude", Name: "Claude", Installed: true, Path: "/bin/claude"},
		{ID: "codex", Name: "Codex", Installed: false},
	}
	got, err := Candidates("", false, found)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "claude" || got[1].ID != "cursor-cli" {
		t.Fatalf("%+v", got)
	}
}

func TestCandidatesNamedAndFallback(t *testing.T) {
	found := []agents.Found{
		{ID: "claude", Name: "Claude", Installed: false},
		{ID: "codex", Name: "Codex", Installed: true, Path: "/bin/codex"},
	}
	if _, err := Candidates("claude", false, found); err == nil {
		t.Fatal("expected missing claude")
	}
	got, err := Candidates("claude", true, found)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "codex" {
		t.Fatalf("%+v", got)
	}
}

func TestCandidatesNamedNoFallback(t *testing.T) {
	found := []agents.Found{
		{ID: "claude", Name: "Claude", Installed: true, Path: "/bin/claude"},
		{ID: "codex", Name: "Codex", Installed: true, Path: "/bin/codex"},
	}
	got, err := Candidates("codex", false, found)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "codex" {
		t.Fatalf("%+v", got)
	}
}

func TestRecipeArgv(t *testing.T) {
	r, err := RecipeOf(agents.Found{ID: "claude", Name: "Claude", Installed: true, Path: "/bin/claude"})
	if err != nil {
		t.Fatal(err)
	}
	args := r.Argv("fix login")
	if r.Mode != ModeCLI || strings.Join(args, " ") != "-p fix login" {
		t.Fatalf("%s %v", r.Mode, args)
	}
	r, err = RecipeOf(agents.Found{ID: "cursor-cli", Name: "Cursor CLI", Installed: true, Path: "/bin/cursor-agent", ACP: "cursor-agent acp"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != ModeACP || strings.Join(r.Args, " ") != "acp" {
		t.Fatalf("%+v", r)
	}
}

func TestCursorAppNotRunnable(t *testing.T) {
	f := agents.Found{ID: "cursor", Name: "Cursor", Installed: true, Path: "/Applications/Cursor.app"}
	if CanRun(f) {
		t.Fatal("cursor app should not be spawnable")
	}
}

func TestFindRunnableFromProbe(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	found := agents.Scan(agents.Probe{Home: home, Path: bin})
	run := Runnable(found)
	if len(run) != 1 || run[0].ID != "claude" {
		t.Fatalf("%+v", run)
	}
}
