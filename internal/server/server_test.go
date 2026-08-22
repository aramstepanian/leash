package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestEmptyTokenDenied(t *testing.T) {
	s := New(config.File{Token: "", WatchRoot: "/proj", Port: 1})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/state", s.auth(s.handleState))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/state", nil)
	req.Header.Set("Authorization", "Bearer anything")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("empty token must deny, got %d", res.StatusCode)
	}
}

func TestWatchMustBeDirectory(t *testing.T) {
	t.Setenv("LEASH_HOME", t.TempDir())
	s := New(config.File{Token: "secret", Port: 1})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/watch", s.auth(s.handleWatch))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/watch", bytes.NewReader([]byte(`{"path":"/no/such/dir"}`)))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("missing dir status %d", res.StatusCode)
	}

	root := t.TempDir()
	body, _ := json.Marshal(map[string]string{"path": root})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/watch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		t.Fatalf("real dir status %d", res.StatusCode)
	}

	var st State
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		res.Body.Close()
		t.Fatal(err)
	}
	res.Body.Close()
	if len(st.WatchRoots) != 1 || st.WatchRoots[0] != root {
		t.Fatalf("watchRoots %+v", st.WatchRoots)
	}

	other := t.TempDir()
	body, _ = json.Marshal(map[string]any{"path": other})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/watch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		res.Body.Close()
		t.Fatal(err)
	}
	res.Body.Close()
	if len(st.WatchRoots) != 2 {
		t.Fatalf("add should keep both folders, got %+v", st.WatchRoots)
	}

	body, _ = json.Marshal(map[string]any{"path": root, "remove": true})
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/v1/watch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if len(st.WatchRoots) != 1 || st.WatchRoots[0] != other {
		t.Fatalf("remove: %+v", st.WatchRoots)
	}
}

func TestTwoAgentsQueue(t *testing.T) {
	t.Setenv("LEASH_HOME", t.TempDir())
	rootA := t.TempDir()
	rootB := t.TempDir()
	s := New(config.File{Token: "t", WatchRoots: []string{rootA, rootB}})
	s.AskTimeout = 3 * time.Second

	doneA := make(chan []byte, 1)
	doneB := make(chan []byte, 1)
	go func() {
		out, err := s.HandleHook(context.Background(), []byte(`{
			"hook_event_name": "beforeShellExecution",
			"command": "rm -rf ./dist",
			"cwd": `+jsonString(rootA)+`
		}`))
		if err != nil {
			doneA <- []byte(err.Error())
			return
		}
		doneA <- out
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.Snapshot().Pending != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s.Snapshot().Pending == nil {
		t.Fatal("first pending never appeared")
	}

	go func() {
		out, err := s.HandleHook(context.Background(), []byte(`{
			"protocol": "leash",
			"hook_event_name": "pre_tool",
			"agent": "OpenCode",
			"cwd": `+jsonString(rootB)+`,
			"tool_name": "bash",
			"tool_input": {"command": "sudo rm -rf /"}
		}`))
		if err != nil {
			doneB <- []byte(err.Error())
			return
		}
		doneB <- out
	}()

	deadline = time.Now().Add(2 * time.Second)
	var st State
	for time.Now().Before(deadline) {
		st = s.Snapshot()
		if st.Waiting == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Waiting != 2 || st.Pending == nil || len(st.Queue) != 1 {
		t.Fatalf("want 2 waiting, got waiting=%d pending=%v queue=%d", st.Waiting, st.Pending, len(st.Queue))
	}
	if st.Pending.Agent != "Cursor" {
		t.Fatalf("oldest should be Cursor, got %q", st.Pending.Agent)
	}
	if st.Queue[0].Agent != "OpenCode" {
		t.Fatalf("queued should be OpenCode, got %q", st.Queue[0].Agent)
	}
	if st.Pending.Root != rootA {
		t.Fatalf("pending root %q want %q", st.Pending.Root, rootA)
	}

	if err := s.Resolve(st.Pending.ID, hookfmt.DecisionKill); err != nil {
		t.Fatal(err)
	}
	outA := <-doneA
	if !bytes.Contains(outA, []byte(`"deny"`)) && !bytes.Contains(outA, []byte(`"permission":"deny"`)) {
		t.Fatalf("cursor deny: %s", outA)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st = s.Snapshot()
		if st.Pending != nil && st.Pending.Agent == "OpenCode" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Pending == nil || st.Pending.Agent != "OpenCode" {
		t.Fatalf("OpenCode should move to the panel, got %+v", st.Pending)
	}
	if st.Waiting != 1 {
		t.Fatalf("waiting %d", st.Waiting)
	}
	if err := s.Resolve(st.Pending.ID, hookfmt.DecisionKill); err != nil {
		t.Fatal(err)
	}
	outB := <-doneB
	if !bytes.Contains(outB, []byte(`"deny"`)) {
		t.Fatalf("opencode deny: %s", outB)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestMissionLoop(t *testing.T) {
	t.Setenv("LEASH_HOME", t.TempDir())
	root := t.TempDir()
	s := New(config.File{Token: "t", WatchRoots: []string{root}})
	s.Auto = func(policy.Assessment) hookfmt.Decision { return hookfmt.DecisionKill }

	if _, err := s.HandleHook(context.Background(), []byte(`{
		"protocol":"leash","hook_event_name":"plan","cwd":`+jsonString(root)+`,
		"agent":"Demo","text":"Fix the login","steps":["read auth.ts","run tests"]
	}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HandleHook(context.Background(), []byte(`{
		"protocol":"leash","hook_event_name":"thought","cwd":`+jsonString(root)+`,
		"agent":"Demo","text":"checking middleware"
	}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HandleHook(context.Background(), []byte(`{
		"cwd":`+jsonString(root)+`,"hook_event_name":"PreToolUse","tool_name":"Bash",
		"tool_input":{"command":"git status"}
	}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HandleHook(context.Background(), []byte(`{
		"protocol":"leash","hook_event_name":"post_tool","cwd":`+jsonString(root)+`,
		"tool_name":"Bash","tool_input":{"command":"npm test"},
		"error":"exit status 1","duration_ms":88
	}`)); err != nil {
		t.Fatal(err)
	}

	st := s.Snapshot()
	if st.Mission.Phase != "failed" {
		t.Fatalf("phase %s", st.Mission.Phase)
	}
	if st.Mission.Failed == nil || st.Mission.Failed.Error == "" {
		t.Fatalf("failed %+v", st.Mission.Failed)
	}
	kinds := map[string]int{}
	for _, e := range st.Mission.Timeline {
		kinds[e.Kind]++
	}
	if kinds["plan"] != 1 || kinds["thought"] != 0 || kinds["error"] < 1 {
		t.Fatalf("timeline kinds %v %+v", kinds, st.Mission.Timeline)
	}
	for _, e := range st.Mission.Timeline {
		if e.Kind == "tool" && strings.Contains(e.Detail, "git status") {
			t.Fatalf("quiet inspection leaked onto the tape: %+v", e)
		}
	}

	s.mission.SetSteer("use bun")
	out, err := s.HandleHook(context.Background(), []byte(`{
		"hook_event_name":"PreToolUse","cwd":`+jsonString(root)+`,
		"tool_name":"Bash","tool_input":{"command":"git status"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("use bun")) && !bytes.Contains(out, []byte("Operator")) {
		t.Fatalf("steer not injected: %s", out)
	}
}

func TestInterruptKillsNextTool(t *testing.T) {
	t.Setenv("LEASH_HOME", t.TempDir())
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.mission.ArmInterrupt()
	out, err := s.HandleHook(context.Background(), []byte(`{
		"cwd":"/proj","hook_event_name":"PreToolUse","tool_name":"Bash",
		"tool_input":{"command":"git status"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("deny")) && !bytes.Contains(out, []byte("Interrupted")) {
		t.Fatalf("want interrupt deny, got %s", out)
	}
}

func TestPendingTitleIsOutcome(t *testing.T) {
	s := New(config.File{Token: "t", WatchRoot: "/proj"})
	s.AskTimeout = 2 * time.Second
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.HandleHook(context.Background(), []byte(`{
			"cwd": "/proj",
			"hook_event_name": "PreToolUse",
			"tool_name": "Bash",
			"tool_input": {"command": "rm -rf ./dist"}
		}`))
	}()
	deadline := time.Now().Add(2 * time.Second)
	var st State
	for time.Now().Before(deadline) {
		st = s.Snapshot()
		if st.Pending != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st.Pending == nil {
		t.Fatal("pending never appeared")
	}
	if st.Pending.Title != "Delete dist" {
		t.Fatalf("title %q", st.Pending.Title)
	}
	_ = s.Resolve(st.Pending.ID, hookfmt.DecisionKill)
	<-done
}

func TestAlwaysRevokeHTTP(t *testing.T) {
	t.Setenv("LEASH_HOME", t.TempDir())
	s := New(config.File{
		Token: "secret",
		Port:  1,
		AlwaysAllow: []policy.Rule{
			{Tool: "Bash", Pattern: "npm test", Root: "/proj"},
		},
	})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/always", s.auth(s.handleAlways))
	mux.HandleFunc("GET /v1/state", s.auth(s.handleState))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	body, _ := json.Marshal(map[string]any{
		"remove": true, "tool": "Bash", "pattern": "npm test", "root": "/proj",
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/always", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	var st State
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if len(st.AlwaysAllow) != 0 {
		t.Fatalf("still have rules %+v", st.AlwaysAllow)
	}
}
