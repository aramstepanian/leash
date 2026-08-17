package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/policy"
)

func TestSilentAllowSafe(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.Auto = func(policy.Assessment) hookfmt.Decision { return hookfmt.DecisionKill }
	out, err := s.HandleHook(context.Background(), []byte(`{
		"cwd": "/proj",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "git status"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("{}")) {
		t.Fatalf("want silent allow, got %s", out)
	}
}

func TestKillDangerous(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.Auto = func(a policy.Assessment) hookfmt.Decision {
		if a.Kind != "destroy" {
			t.Fatalf("kind %s", a.Kind)
		}
		return hookfmt.DecisionKill
	}
	out, err := s.HandleHook(context.Background(), []byte(`{
		"cwd": "/proj",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf /tmp/build"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	h := parsed["hookSpecificOutput"].(map[string]any)
	if h["permissionDecision"] != "deny" {
		t.Fatalf("%s", out)
	}
}

func TestCursorDialectDeny(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.Auto = func(policy.Assessment) hookfmt.Decision { return hookfmt.DecisionKill }
	out, err := s.HandleHook(context.Background(), []byte(`{
		"hook_event_name": "beforeShellExecution",
		"command": "rm -rf ./dist",
		"cwd": "/proj"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["permission"] != "deny" {
		t.Fatalf("%s", out)
	}
}

func TestGenericProtocolDeny(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.Auto = func(policy.Assessment) hookfmt.Decision { return hookfmt.DecisionKill }
	out, err := s.HandleHook(context.Background(), []byte(`{
		"protocol": "leash",
		"hook_event_name": "pre_tool",
		"cwd": "/proj",
		"tool_name": "bash",
		"tool_input": {"command": "sudo rm -rf /"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["decision"] != "deny" {
		t.Fatalf("%s", out)
	}
}

func TestDedupeDoubleHook(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	calls := 0
	s.Auto = func(policy.Assessment) hookfmt.Decision {
		calls++
		return hookfmt.DecisionKill
	}
	body := []byte(`{
		"hook_event_name": "preToolUse",
		"tool_name": "Shell",
		"tool_input": {"command": "rm -rf ./dist", "working_directory": "/proj"}
	}`)
	if _, err := s.HandleHook(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HandleHook(context.Background(), body); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("asked %d times, want 1", calls)
	}
}

func TestUndoAfterWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "app.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(config.File{Token: "t", WatchRoot: root})
	_, err := s.HandleHook(context.Background(), []byte(`{
		"cwd": "`+root+`",
		"hook_event_name": "PreToolUse",
		"tool_name": "Write",
		"tool_input": {"file_path": "`+path+`", "content": "new"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := s.Bursts.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("restored %d", n)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatalf("got %q", got)
	}
}

func TestHTTPAuthAndDecision(t *testing.T) {
	s := New(config.File{Token: "secret", WatchRoot: "/proj", Port: 1})
	s.AskTimeout = 2 * time.Second
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/hook", s.auth(s.handleHook))
	mux.HandleFunc("POST /v1/decision", s.auth(s.handleDecision))
	mux.HandleFunc("GET /v1/state", s.auth(s.handleState))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != 401 {
		t.Fatalf("status %d", res.StatusCode)
	}
	res.Body.Close()

	done := make(chan []byte, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/hook", bytes.NewReader([]byte(`{
			"cwd": "/proj",
			"hook_event_name": "PreToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "rm -rf ./dist"}
		}`)))
		req.Header.Set("Authorization", "Bearer secret")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- []byte(err.Error())
			return
		}
		defer res.Body.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(res.Body)
		done <- buf.Bytes()
	}()

	deadline := time.Now().Add(2 * time.Second)
	var id string
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
		req.Header.Set("Authorization", "Bearer secret")
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			var st State
			_ = json.NewDecoder(res.Body).Decode(&st)
			res.Body.Close()
			if st.Pending != nil {
				id = st.Pending.ID
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("pending never appeared")
	}
	body, _ := json.Marshal(map[string]string{"id": id, "action": "kill"})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/decision", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	out := <-done
	if !bytes.Contains(out, []byte(`"deny"`)) {
		t.Fatalf("got %s", out)
	}
}
