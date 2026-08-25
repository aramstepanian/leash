package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/leashapp/leash/internal/hookfmt"
)

const clientName = "leash"

// ServePrompt is a tiny ACP client: initialize, one session, one prompt.
// It answers session/request_permission through Gate and forwards
// session/update through Notify. It does not render a chat transcript.
func ServePrompt(ctx context.Context, agentIn io.Writer, agentOut io.Reader, agent, cwd, text string, gate Gate, notify Notify) error {
	if gate == nil {
		gate = func(context.Context, hookfmt.Event) hookfmt.Decision { return hookfmt.DecisionAllow }
	}
	h := &host{
		ctx:      ctx,
		agent:    agent,
		cwd:      cwd,
		gate:     gate,
		notify:   notify,
		agentIn:  agentIn,
		agentOut: bufio.NewReaderSize(agentOut, 64<<10),
		pending:  map[int]chan frame{},
	}
	readErr := make(chan error, 1)
	go func() {
		err := h.readLoop()
		h.failPending(err)
		readErr <- err
	}()

	runErr := make(chan error, 1)
	go func() { runErr <- h.run(text) }()

	select {
	case err := <-runErr:
		if h.ctx.Err() != nil {
			h.cancelSession()
			return h.ctx.Err()
		}
		return err
	case err := <-readErr:
		if err == io.EOF {
			select {
			case run := <-runErr:
				return run
			default:
				return fmt.Errorf("%s closed the ACP stream", agent)
			}
		}
		return err
	case <-ctx.Done():
		h.cancelSession()
		return ctx.Err()
	}
}

func (h *host) run(text string) error {
	if err := h.initialize(); err != nil {
		return err
	}
	sid, err := h.sessionNew(h.cwd)
	if err != nil {
		return err
	}
	h.session.Store(sid)
	return h.sessionPrompt(sid, text)
}

type host struct {
	ctx      context.Context
	agent    string
	cwd      string
	gate     Gate
	notify   Notify
	agentIn  io.Writer
	agentOut *bufio.Reader

	writeMu sync.Mutex
	pendMu  sync.Mutex
	pending map[int]chan frame
	nextID  int

	session atomic.Value // string
}

func (h *host) initialize() error {
	f, err := h.call("initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": clientName, "version": "0.8.0"},
	})
	if err != nil {
		return err
	}
	var res struct {
		AuthMethods []struct {
			ID string `json:"id"`
		} `json:"authMethods"`
	}
	_ = json.Unmarshal(f.Result, &res)
	if len(res.AuthMethods) > 0 {
		return fmt.Errorf("%s requires authentication — start it from its own app", h.agent)
	}
	return nil
}

func (h *host) sessionNew(cwd string) (string, error) {
	f, err := h.call("session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
	})
	if err != nil {
		return "", err
	}
	var res struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(f.Result, &res); err != nil || res.SessionID == "" {
		return "", fmt.Errorf("session/new: missing sessionId")
	}
	return res.SessionID, nil
}

func (h *host) sessionPrompt(sid, text string) error {
	f, err := h.call("session/prompt", map[string]any{
		"sessionId": sid,
		"prompt":    []any{map[string]any{"type": "text", "text": text}},
	})
	if err != nil {
		return err
	}
	var res struct {
		StopReason string `json:"stopReason"`
	}
	_ = json.Unmarshal(f.Result, &res)
	if res.StopReason == "cancelled" && h.ctx.Err() != nil {
		return h.ctx.Err()
	}
	return nil
}

func (h *host) cancelSession() {
	sid, _ := h.session.Load().(string)
	if sid == "" {
		return
	}
	_ = h.notifyAgent("session/cancel", map[string]any{"sessionId": sid})
}

func (h *host) failPending(err error) {
	msg := "agent closed"
	if err != nil && err != io.EOF {
		msg = err.Error()
	}
	raw := mustRaw(map[string]any{"code": -32603, "message": msg})
	h.pendMu.Lock()
	defer h.pendMu.Unlock()
	for id, ch := range h.pending {
		select {
		case ch <- frame{Error: raw}:
		default:
		}
		delete(h.pending, id)
	}
}

func (h *host) call(method string, params any) (frame, error) {
	id := h.alloc()
	ch := make(chan frame, 1)
	h.pendMu.Lock()
	h.pending[id] = ch
	h.pendMu.Unlock()
	defer func() {
		h.pendMu.Lock()
		delete(h.pending, id)
		h.pendMu.Unlock()
	}()

	raw, err := json.Marshal(frame{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
		Params:  mustRaw(params),
	})
	if err != nil {
		return frame{}, err
	}
	if err := h.write(raw); err != nil {
		return frame{}, err
	}
	select {
	case <-h.ctx.Done():
		return frame{}, h.ctx.Err()
	case f := <-ch:
		if len(f.Error) > 0 && string(f.Error) != "null" {
			return frame{}, fmt.Errorf("%s: %s", method, truncateErr(f.Error))
		}
		return f, nil
	}
}

func (h *host) notifyAgent(method string, params any) error {
	raw, err := json.Marshal(frame{
		JSONRPC: "2.0",
		Method:  method,
		Params:  mustRaw(params),
	})
	if err != nil {
		return err
	}
	return h.write(raw)
}

func (h *host) alloc() int {
	h.pendMu.Lock()
	defer h.pendMu.Unlock()
	h.nextID++
	return h.nextID
}

func (h *host) write(raw []byte) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	return writeFrame(h.agentIn, raw)
}

func (h *host) readLoop() error {
	for {
		raw, err := readFrame(h.agentOut)
		if err != nil {
			return err
		}
		var f frame
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		switch {
		case f.isRequest() && f.Method == "session/request_permission":
			if err := h.answerPermission(f); err != nil {
				return err
			}
		case f.isRequest():
			_ = h.replyError(f.ID, -32601, "Method not found")
		case f.Method == "session/update":
			if h.notify != nil {
				if ev, ok := eventFromUpdate(h.agent, h.cwd, f.Params); ok {
					go h.notify(ev)
				}
			}
		default:
			id := intID(f.ID)
			if id == 0 {
				continue
			}
			h.pendMu.Lock()
			ch := h.pending[id]
			h.pendMu.Unlock()
			if ch != nil {
				select {
				case ch <- f:
				default:
				}
			}
		}
	}
}

func (h *host) answerPermission(f frame) error {
	ev := EventFromPermission(h.agent, h.cwd, f.Params)
	d := h.gate(h.ctx, ev)
	id, cancelled := PickOption(f.Params, d)
	reply := frame{
		JSONRPC: "2.0",
		ID:      f.ID,
		Result:  permissionResult(id, cancelled),
	}
	raw, err := json.Marshal(reply)
	if err != nil {
		return err
	}
	return h.write(raw)
}

func (h *host) replyError(id json.RawMessage, code int, msg string) error {
	raw, err := json.Marshal(frame{
		JSONRPC: "2.0",
		ID:      id,
		Error:   mustRaw(map[string]any{"code": code, "message": msg}),
	})
	if err != nil {
		return err
	}
	return h.write(raw)
}

func mustRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

func intID(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return int(f)
	}
	return 0
}

func truncateErr(raw json.RawMessage) string {
	s := string(raw)
	if len(s) > 240 {
		return s[:240] + "…"
	}
	return s
}
