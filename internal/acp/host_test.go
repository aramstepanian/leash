package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/leashapp/leash/internal/hookfmt"
)

func TestServePromptGatesAndNotifies(t *testing.T) {
	fromHost, toAgent := io.Pipe()
	fromAgent, toHost := io.Pipe()
	defer fromHost.Close()
	defer toAgent.Close()
	defer fromAgent.Close()
	defer toHost.Close()

	var gated hookfmt.Event
	gate := func(_ context.Context, ev hookfmt.Event) hookfmt.Decision {
		gated = ev
		return hookfmt.DecisionKill
	}
	var notes []hookfmt.Event
	var mu sync.Mutex
	notify := func(ev hookfmt.Event) {
		mu.Lock()
		notes = append(notes, ev)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	errCh := make(chan error, 2)
	go func() {
		errCh <- ServePrompt(ctx, toAgent, fromAgent, "Cursor", "/proj", "fix login", gate, notify)
	}()
	go func() {
		errCh <- fakeACP(fromHost, toHost)
	}()

	var err error
	select {
	case err = <-errCh:
	case <-ctx.Done():
		t.Fatal("timeout")
	}
	if err != nil {
		t.Fatal(err)
	}
	if gated.ToolName != "Bash" || gated.ToolInput["command"] != "rm -rf ./dist" {
		t.Fatalf("gate %+v", gated)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(notes)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(notes) == 0 || notes[0].HookEventName != "plan" {
		t.Fatalf("notes %+v", notes)
	}
}

func fakeACP(in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		raw, err := readFrame(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var f frame
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		switch f.Method {
		case "initialize":
			if err := writeResult(out, f.ID, map[string]any{"protocolVersion": 1, "authMethods": []any{}}); err != nil {
				return err
			}
		case "session/new":
			if err := writeResult(out, f.ID, map[string]any{"sessionId": "s1"}); err != nil {
				return err
			}
		case "session/prompt":
			plan := frame{
				JSONRPC: "2.0",
				Method:  "session/update",
				Params: mustRaw(map[string]any{
					"sessionId": "s1",
					"update": map[string]any{
						"sessionUpdate": "plan",
						"entries":       []any{map[string]any{"content": "read files", "status": "pending"}},
					},
				}),
			}
			b, _ := json.Marshal(plan)
			if err := writeFrame(out, b); err != nil {
				return err
			}
			perm := frame{
				JSONRPC: "2.0",
				ID:      json.RawMessage("99"),
				Method:  "session/request_permission",
				Params: mustRaw(map[string]any{
					"sessionId": "s1",
					"toolCall": map[string]any{
						"title":    "rm -rf ./dist",
						"kind":     "execute",
						"rawInput": map[string]any{"command": "rm -rf ./dist"},
					},
					"options": []any{
						map[string]any{"optionId": "allow-once", "kind": "allow_once"},
						map[string]any{"optionId": "reject-once", "kind": "reject_once"},
					},
				}),
			}
			b, _ = json.Marshal(perm)
			if err := writeFrame(out, b); err != nil {
				return err
			}
			if _, err := readFrame(r); err != nil {
				return err
			}
			if err := writeResult(out, f.ID, map[string]any{"stopReason": "end_turn"}); err != nil {
				return err
			}
			return nil
		}
	}
}

func writeResult(w io.Writer, id json.RawMessage, result any) error {
	raw, err := json.Marshal(frame{JSONRPC: "2.0", ID: id, Result: mustRaw(result)})
	if err != nil {
		return err
	}
	return writeFrame(w, raw)
}
