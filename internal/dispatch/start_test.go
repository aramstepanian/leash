package dispatch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartCLIWritesPrompt(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$LEASH_OUT\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "args")
	t.Setenv("LEASH_OUT", out)
	r := Recipe{ID: "claude", Name: "Claude", Mode: ModeCLI, Command: bin, Args: []string{"-p"}}
	var log bytes.Buffer
	if err := Start(context.Background(), r, dir, "fix the login", nil, nil, &log); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "-p\nfix the login" && strings.TrimSpace(string(got)) != "-p fix the login" {
		if !strings.Contains(string(got), "fix the login") || !strings.Contains(string(got), "-p") {
			t.Fatalf("args %q", got)
		}
	}
}

func TestStartCLICancelKills(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "sleepagent")
	script := "#!/bin/sh\ntrap 'exit 0' TERM\nwhile true; do sleep 0.05; done\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := Recipe{ID: "claude", Name: "Claude", Mode: ModeCLI, Command: bin, Args: []string{"-p"}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, r, dir, "hang", nil, nil, nil)
	}()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want cancel error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("agent was not killed")
	}
}

func TestStartMissingBinary(t *testing.T) {
	r := Recipe{ID: "claude", Name: "Claude", Mode: ModeCLI, Command: "/no/such/claude", Args: []string{"-p"}}
	err := Start(context.Background(), r, t.TempDir(), "x", nil, nil, nil)
	if !IsStartError(err) {
		t.Fatalf("want StartError, got %v", err)
	}
}
