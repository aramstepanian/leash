package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/hookfmt"
)

func TestEventFromPermissionShell(t *testing.T) {
	raw := []byte(`{
		"sessionId": "s1",
		"toolCall": {
			"toolCallId": "c1",
			"title": "rm -rf ./dist",
			"kind": "execute",
			"rawInput": {"command": "rm -rf ./dist"}
		},
		"options": [
			{"optionId": "allow-once", "name": "Allow", "kind": "allow_once"},
			{"optionId": "reject-once", "name": "Reject", "kind": "reject_once"}
		]
	}`)
	ev := EventFromPermission("Cursor", "/proj", raw)
	if ev.ToolName != "Bash" || ev.ToolInput["command"] != "rm -rf ./dist" {
		t.Fatalf("%+v", ev)
	}
	if ev.CWD != "/proj" || ev.Agent != "Cursor" {
		t.Fatalf("%+v", ev)
	}
	id, cancelled := PickOption(raw, hookfmt.DecisionKill)
	if cancelled || id != "reject-once" {
		t.Fatalf("kill -> %s cancelled=%v", id, cancelled)
	}
	id, cancelled = PickOption(raw, hookfmt.DecisionAllow)
	if cancelled || id != "allow-once" {
		t.Fatalf("allow -> %s", id)
	}
}

func TestEventFromPermissionEditPath(t *testing.T) {
	raw := []byte(`{
		"toolCall": {
			"kind": "edit",
			"locations": [{"path": "/proj/.env"}]
		},
		"options": [{"optionId": "a", "kind": "allow_once"}]
	}`)
	ev := EventFromPermission("OpenCode", "/proj", raw)
	if ev.ToolName != "Edit" || ev.ToolInput["file_path"] != "/proj/.env" {
		t.Fatalf("%+v", ev)
	}
}

func TestProxyInterceptsPermission(t *testing.T) {
	clientRead, clientWrite := io.Pipe()
	toClient, clientOut := io.Pipe()
	fromProxy, toAgent := io.Pipe()
	fromAgent, agentWrite := io.Pipe()

	var got hookfmt.Event
	gate := func(_ context.Context, ev hookfmt.Event) hookfmt.Decision {
		got = ev
		return hookfmt.DecisionKill
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- Proxy(ctx, clientRead, clientOut, toAgent, fromAgent, "Cursor", gate, nil)
	}()

	init := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")
	if _, err := clientWrite.Write(init); err != nil {
		t.Fatal(err)
	}
	gotInit := readLine(t, fromProxy)
	if !bytes.Contains(gotInit, []byte(`"initialize"`)) {
		t.Fatalf("forward initialize: %s", gotInit)
	}

	perm := []byte(`{"jsonrpc":"2.0","id":5,"method":"session/request_permission","params":{"sessionId":"s","toolCall":{"kind":"execute","title":"rm -rf /tmp/x","rawInput":{"command":"rm -rf /tmp/x"}},"options":[{"optionId":"allow-once","kind":"allow_once"},{"optionId":"reject-once","kind":"reject_once"}]}}` + "\n")
	if _, err := agentWrite.Write(perm); err != nil {
		t.Fatal(err)
	}
	reply := readLine(t, fromProxy)
	var f frame
	if err := json.Unmarshal(reply, &f); err != nil {
		t.Fatal(err, string(reply))
	}
	if string(f.ID) != "5" {
		t.Fatalf("id %s", f.ID)
	}
	if !bytes.Contains(f.Result, []byte(`"reject-once"`)) {
		t.Fatalf("result %s", f.Result)
	}
	if got.ToolInput["command"] != "rm -rf /tmp/x" {
		t.Fatalf("gate event %+v", got)
	}

	clientSaw := make(chan []byte, 1)
	go func() {
		b := make([]byte, 4096)
		n, _ := toClient.Read(b)
		clientSaw <- append([]byte{}, b[:n]...)
	}()
	select {
	case leak := <-clientSaw:
		t.Fatalf("client saw %s", leak)
	case <-time.After(80 * time.Millisecond):
	}

	cancel()
	_ = clientWrite.Close()
	_ = agentWrite.Close()
	select {
	case <-errCh:
	case <-time.After(time.Second):
	}
}

func TestProxyForwardsSessionNewCwd(t *testing.T) {
	clientRead, clientWrite := io.Pipe()
	_, clientOut := io.Pipe()
	fromProxy, toAgent := io.Pipe()
	fromAgent, agentWrite := io.Pipe()

	var mu sync.Mutex
	var cwd string
	gate := func(_ context.Context, ev hookfmt.Event) hookfmt.Decision {
		mu.Lock()
		cwd = ev.CWD
		mu.Unlock()
		return hookfmt.DecisionAllow
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { _ = Proxy(ctx, clientRead, clientOut, toAgent, fromAgent, "Hermes", gate, nil) }()

	_, _ = clientWrite.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/work/app"}}` + "\n"))
	_ = readLine(t, fromProxy)

	_, _ = agentWrite.Write([]byte(`{"jsonrpc":"2.0","id":9,"method":"session/request_permission","params":{"toolCall":{"kind":"execute","rawInput":{"command":"git status"}},"options":[{"optionId":"a","kind":"allow_once"}]}}` + "\n"))
	_ = readLine(t, fromProxy)

	mu.Lock()
	got := cwd
	mu.Unlock()
	if got != "/work/app" {
		t.Fatalf("cwd %q", got)
	}
	cancel()
	_ = clientWrite.Close()
	_ = agentWrite.Close()
}

func TestResolveLaunchAlias(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "cursor-agent"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launch, err := ResolveLaunch([]string{"cursor"}, agents.Probe{Home: home, Path: bin})
	if err != nil {
		t.Fatal(err)
	}
	if launch.Name != "Cursor" || len(launch.Args) != 1 || launch.Args[0] != "acp" {
		t.Fatalf("%+v", launch)
	}
	if launch.Command != filepath.Join(bin, "cursor-agent") {
		t.Fatalf("command %s", launch.Command)
	}
}

func readLine(t *testing.T, r io.Reader) []byte {
	t.Helper()
	buf := make([]byte, 0, 1024)
	one := make([]byte, 1)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, err := r.Read(one)
		if n == 1 {
			buf = append(buf, one[0])
			if one[0] == '\n' {
				return buf
			}
			continue
		}
		if err == io.EOF {
			return buf
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout reading line, got %s", buf)
	return nil
}
