package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leashapp/leash/internal/agents"
)

func TestPickPrefersOpenCode(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "opencode"))
	writeExe(t, filepath.Join(bin, "claude"))
	p := agents.Probe{Home: home, Path: bin}
	got, err := Pick(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "opencode" {
		t.Fatalf("got %+v", got)
	}
	rec, err := For(got, "fix the flaky test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ACP || len(rec.Args) < 2 || rec.Args[0] != "run" {
		t.Fatalf("%+v", rec)
	}
}

func TestPickNamedAndSkipApp(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "claude"))
	p := agents.Probe{Home: home, Path: bin}
	if _, err := Pick(p, "cursor"); err == nil {
		t.Fatal("cursor.app is not a CLI runner")
	}
	got, err := Pick(p, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "claude" {
		t.Fatalf("%+v", got)
	}
}

func TestRunPrintAgent(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	root := t.TempDir()
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "opencode"), "#!/bin/sh\necho ran-ok\n")
	p := agents.Probe{Home: home, Path: bin}
	name, out, err := RunWith(context.Background(), Job{Prompt: "fix the flaky test", Root: root}, p)
	if err != nil {
		t.Fatal(err)
	}
	if name != "OpenCode" {
		t.Fatalf("agent %s", name)
	}
	if !strings.Contains(out, "ran-ok") {
		t.Fatalf("result %q", out)
	}
}

func TestPickAllowPrefersOnce(t *testing.T) {
	opt, ok := pickAllow([]byte(`{"options":[{"optionId":"deny","kind":"reject_once"},{"optionId":"ok","kind":"allow_once"}]}`))
	if !ok || opt != "ok" {
		t.Fatalf("got %q %v", opt, ok)
	}
}

func writeExe(t *testing.T, path string, body ...string) {
	t.Helper()
	script := "#!/bin/sh\nexit 0\n"
	if len(body) > 0 {
		script = body[0]
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
