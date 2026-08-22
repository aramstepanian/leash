package dispatch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func oneShotACP(ctx context.Context, command string, args []string, cwd, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	c := &acpClient{
		ctx:    ctx,
		w:      stdin,
		r:      bufio.NewReaderSize(stdout, 64<<10),
		wait:   map[string]chan rpc{},
		chunks: &strings.Builder{},
	}
	go c.readLoop()

	if _, err := c.call("initialize", map[string]any{
		"protocolVersion": 1,
		"clientInfo":      map[string]string{"name": "leash", "version": "0.8.0"},
		"capabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
	}); err != nil {
		_ = stdin.Close()
		return "", fmt.Errorf("acp initialize: %w %s", err, stderr.String())
	}
	c.notify("initialized", map[string]any{})

	raw, err := c.call("session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})
	if err != nil {
		_ = stdin.Close()
		return "", fmt.Errorf("session/new: %w %s", err, stderr.String())
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(raw, &created)
	if created.SessionID == "" {
		_ = stdin.Close()
		return "", fmt.Errorf("session/new: no sessionId %s", string(raw))
	}

	_, err = c.call("session/prompt", map[string]any{
		"sessionId": created.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": prompt}},
	})
	_ = stdin.Close()
	select {
	case <-ctx.Done():
		return c.chunks.String(), ctx.Err()
	case waitErr := <-done:
		if err != nil {
			return c.chunks.String(), fmt.Errorf("session/prompt: %w %s", err, stderr.String())
		}
		if waitErr != nil && ctx.Err() == nil {
			return c.chunks.String(), waitErr
		}
		return c.chunks.String(), nil
	}
}

type acpClient struct {
	ctx    context.Context
	w      io.WriteCloser
	r      *bufio.Reader
	mu     sync.Mutex
	next   int
	wait   map[string]chan rpc
	chunks *strings.Builder
}

func (c *acpClient) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := make(chan rpc, 1)
	key := fmt.Sprintf("%d", id)
	c.wait[key] = ch
	c.mu.Unlock()

	body, err := json.Marshal(rpc{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
		Params:  mustJSON(params),
	})
	if err != nil {
		return nil, err
	}
	if err := c.write(body); err != nil {
		return nil, err
	}
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	case fr := <-ch:
		if len(fr.Error) > 0 && string(fr.Error) != "null" {
			return nil, fmt.Errorf("%s", string(fr.Error))
		}
		return fr.Result, nil
	}
}

func (c *acpClient) notify(method string, params any) {
	body, _ := json.Marshal(rpc{
		JSONRPC: "2.0",
		Method:  method,
		Params:  mustJSON(params),
	})
	_ = c.write(body)
}

func (c *acpClient) write(body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.w.Write(append(body, '\n'))
	return err
}

func (c *acpClient) readLoop() {
	for {
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			return
		}
		line = bytesTrim(line)
		if len(line) == 0 {
			continue
		}
		var fr rpc
		if json.Unmarshal(line, &fr) != nil {
			continue
		}
		if fr.Method == "session/request_permission" && len(fr.ID) > 0 {
			opt, _ := pickAllow(fr.Params)
			_ = c.write(mustJSON(rpc{
				JSONRPC: "2.0",
				ID:      fr.ID,
				Result:  mustJSON(map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": opt}}),
			}))
			continue
		}
		if fr.Method == "session/update" {
			c.noteUpdate(fr.Params)
			continue
		}
		if fr.Method != "" && len(fr.ID) > 0 {
			_ = c.write(mustJSON(rpc{
				JSONRPC: "2.0",
				ID:      fr.ID,
				Result:  mustJSON(map[string]any{}),
			}))
			continue
		}
		if len(fr.ID) == 0 {
			continue
		}
		key := strings.Trim(string(fr.ID), `"`)
		c.mu.Lock()
		ch := c.wait[key]
		c.mu.Unlock()
		if ch != nil {
			ch <- fr
		}
	}
}

func (c *acpClient) noteUpdate(params json.RawMessage) {
	var wrap struct {
		Update json.RawMessage `json:"update"`
	}
	if json.Unmarshal(params, &wrap) != nil || len(wrap.Update) == 0 {
		wrap.Update = params
	}
	var u struct {
		SessionUpdate string          `json:"sessionUpdate"`
		Content       json.RawMessage `json:"content"`
		Text          string          `json:"text"`
	}
	if json.Unmarshal(wrap.Update, &u) != nil {
		return
	}
	if u.SessionUpdate != "agent_message_chunk" && u.SessionUpdate != "agent_message" {
		return
	}
	text := u.Text
	if text == "" && len(u.Content) > 0 {
		var block struct {
			Text string `json:"text"`
			Type string `json:"type"`
		}
		_ = json.Unmarshal(u.Content, &block)
		text = block.Text
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	c.mu.Lock()
	c.chunks.WriteString(text)
	c.mu.Unlock()
}

func pickAllow(params json.RawMessage) (string, bool) {
	var p struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	_ = json.Unmarshal(params, &p)
	for _, kind := range []string{"allow_once", "allow_always"} {
		for _, opt := range p.Options {
			if opt.Kind == kind {
				return opt.OptionID, true
			}
		}
	}
	if len(p.Options) > 0 {
		return p.Options[0].OptionID, true
	}
	return "allow-once", false
}

func mustJSON(v any) json.RawMessage {
	if raw, ok := v.(json.RawMessage); ok {
		return raw
	}
	b, _ := json.Marshal(v)
	return b
}

func bytesTrim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
