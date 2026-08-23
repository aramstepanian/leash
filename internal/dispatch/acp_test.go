package dispatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestACPStreamsTextAndExits(t *testing.T) {
	bin := buildFakeACP(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var live string
	start := time.Now()
	out, err := oneShotACP(ctx, bin, nil, t.TempDir(), "hello", func(s string) { live = s })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hello from ACP.") {
		t.Fatalf("result %q", out)
	}
	if !strings.Contains(live, "Hello from ACP.") {
		t.Fatalf("live %q", live)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatalf("ACP agent was not killed: %s", time.Since(start))
	}
}

func TestACPInitializeFailsFast(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "dead")
	writeExe(t, bin, "#!/bin/sh\necho not-json\nexit 0\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := oneShotACP(ctx, bin, nil, dir, "hello", nil)
	if err == nil {
		t.Fatal("want initialize error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("hung on initialize: %s", time.Since(start))
	}
}

func TestACPRateLimitFailsFast(t *testing.T) {
	t.Setenv("LEASH_ACP_FAIL", "rate")
	bin := buildFakeACP(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := oneShotACP(ctx, bin, nil, t.TempDir(), "hello", nil)
	if err == nil || !strings.Contains(err.Error(), "rate-limited") {
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("hung: %s", time.Since(start))
	}
}

const fakeACPSrc = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var msg map[string]any
		if json.Unmarshal(sc.Bytes(), &msg) != nil {
			continue
		}
		method, _ := msg["method"].(string)
		id := msg["id"]
		switch method {
		case "initialize":
			reply(id, map[string]any{"protocolVersion": 1})
		case "session/new":
			reply(id, map[string]any{"sessionId": "s1"})
		case "session/prompt":
			if os.Getenv("LEASH_ACP_FAIL") == "rate" {
				fmt.Fprintln(os.Stderr, "Rate limit exceeded. Please try again later.")
				select {}
			}
			note("session/update", map[string]any{
				"sessionId": "s1",
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]any{"type": "text", "text": "Hello from ACP."},
				},
			})
			reply(id, map[string]any{"stopReason": "end_turn"})
		}
	}
}

func reply(id any, result any) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func note(method string, params any) {
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
`

func buildFakeACP(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fakeACPSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "fakeacp")
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake acp: %v\n%s", err, out)
	}
	return bin
}
