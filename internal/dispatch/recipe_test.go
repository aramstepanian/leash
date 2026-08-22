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
	if !rec.JSON {
		t.Fatalf("opencode should request json: %+v", rec)
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

func TestStripANSI(t *testing.T) {
	raw := "\x1b[0m\n> build · nemotron\n\x1b[0m$ \x1b[0mls -la\ntotal 40\n"
	got := stripANSI(raw)
	if strings.Contains(got, "[0m") || strings.Contains(got, "\x1b") {
		t.Fatalf("ansi left in %q", got)
	}
	if !strings.Contains(got, "ls -la") || !strings.Contains(got, "total 40") {
		t.Fatalf("lost output: %q", got)
	}
	orphan := stripANSI("[0m$ [0mls -la")
	if strings.Contains(orphan, "[0m") {
		t.Fatalf("orphan sgr left in %q", orphan)
	}
}

func TestPickCursorAlias(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "cursor-agent"))
	p := agents.Probe{Home: home, Path: bin}
	got, err := Pick(p, "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "cursor-cli" {
		t.Fatalf("%+v", got)
	}
}

func TestOpenCodeTextAndChrome(t *testing.T) {
	jsonl := "{\"type\":\"text\",\"part\":{\"type\":\"text\",\"text\":\"Hello from the agent.\"}}\n"
	if got := openCodeText(jsonl); got != "Hello from the agent." {
		t.Fatalf("json text %q", got)
	}
	raw := "> build · nemotron-3-ultra-free\n$ ls -la\nHello! How can I help you today?\n"
	got := extractReply(raw)
	if strings.Contains(got, "build ·") || strings.Contains(got, "$ ls") {
		t.Fatalf("chrome left in %q", got)
	}
	if !strings.Contains(got, "Hello!") {
		t.Fatalf("lost reply %q", got)
	}
}

func TestFindCursorCLILooksForAgent(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	writeExe(t, filepath.Join(bin, "agent"))
	got := agents.FindCursorCLI(agents.Probe{Home: home, Path: "/nonexistent"})
	if got == "" {
		t.Fatal("expected agent binary")
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
