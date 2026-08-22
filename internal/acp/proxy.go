package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/leashapp/leash/internal/hookfmt"
)

const maxFrame = 1 << 20

type frame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (f frame) isRequest() bool { return f.Method != "" && len(f.ID) > 0 }

// Gate decides Allow / Always / Kill for one ACP permission request.
// A nil gate allows.
type Gate func(ctx context.Context, ev hookfmt.Event) hookfmt.Decision

// Notify is a non-blocking hook for plan / completed tool updates.
type Notify func(ev hookfmt.Event)

// Proxy sits on ACP stdio. It forwards every frame except
// session/request_permission, which it answers through Gate (the Leash HUD).
func Proxy(ctx context.Context, clientIn io.Reader, clientOut io.Writer, agentIn io.Writer, agentOut io.Reader, agent string, gate Gate, notify Notify) error {
	if gate == nil {
		gate = func(context.Context, hookfmt.Event) hookfmt.Decision { return hookfmt.DecisionAllow }
	}
	p := &proxy{
		ctx:       ctx,
		clientIn:  bufio.NewReaderSize(clientIn, 64<<10),
		clientOut: clientOut,
		agentIn:   agentIn,
		agentOut:  bufio.NewReaderSize(agentOut, 64<<10),
		agent:     agent,
		gate:      gate,
		notify:    notify,
	}

	errCh := make(chan error, 2)
	go func() { errCh <- p.clientToAgent() }()
	go func() { errCh <- p.agentToClient() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

type proxy struct {
	ctx       context.Context
	clientIn  *bufio.Reader
	clientOut io.Writer
	agentIn   io.Writer
	agentOut  *bufio.Reader
	agent     string
	cwd       string
	gate      Gate
	notify    Notify

	clientMu sync.Mutex
	agentMu  sync.Mutex
	cwdMu    sync.Mutex
}

func (p *proxy) setCwd(cwd string) {
	if cwd == "" {
		return
	}
	p.cwdMu.Lock()
	p.cwd = cwd
	p.cwdMu.Unlock()
}

func (p *proxy) getCwd() string {
	p.cwdMu.Lock()
	defer p.cwdMu.Unlock()
	return p.cwd
}

func (p *proxy) clientToAgent() error {
	for {
		raw, err := readFrame(p.clientIn)
		if err != nil {
			return err
		}
		var f frame
		if json.Unmarshal(raw, &f) == nil {
			switch f.Method {
			case "session/new", "session/load":
				p.setCwd(cwdFromParams(f.Params))
			}
		}
		if err := p.writeAgent(raw); err != nil {
			return err
		}
	}
}

func (p *proxy) agentToClient() error {
	for {
		raw, err := readFrame(p.agentOut)
		if err != nil {
			return err
		}
		var f frame
		if err := json.Unmarshal(raw, &f); err != nil {
			if err := p.writeClient(raw); err != nil {
				return err
			}
			continue
		}
		switch {
		case f.isRequest() && f.Method == "session/request_permission":
			if err := p.answerPermission(f); err != nil {
				return err
			}
		default:
			if f.Method == "session/update" && p.notify != nil {
				if ev, ok := eventFromUpdate(p.agent, p.getCwd(), f.Params); ok {
					go p.notify(ev)
				}
			}
			if err := p.writeClient(raw); err != nil {
				return err
			}
		}
	}
}

func (p *proxy) answerPermission(f frame) error {
	ev := EventFromPermission(p.agent, p.getCwd(), f.Params)
	d := p.gate(p.ctx, ev)
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
	return p.writeAgent(raw)
}

func (p *proxy) writeAgent(raw []byte) error {
	p.agentMu.Lock()
	defer p.agentMu.Unlock()
	return writeFrame(p.agentIn, raw)
}

func (p *proxy) writeClient(raw []byte) error {
	p.clientMu.Lock()
	defer p.clientMu.Unlock()
	return writeFrame(p.clientOut, raw)
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	line = bytes.TrimRight(line, "\r\n")
	if len(line) > maxFrame {
		return nil, fmt.Errorf("acp frame too large")
	}
	if err == io.EOF {
		if len(line) == 0 {
			return nil, io.EOF
		}
		return line, nil
	}
	if err != nil {
		return nil, err
	}
	if len(line) == 0 {
		return readFrame(r)
	}
	return line, nil
}

func writeFrame(w io.Writer, raw []byte) error {
	raw = bytes.TrimRight(raw, "\r\n")
	_, err := w.Write(append(append([]byte{}, raw...), '\n'))
	return err
}
