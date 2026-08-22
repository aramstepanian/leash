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
	"time"

	"github.com/leashapp/leash/internal/version"
)

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func oneShotACP(ctx context.Context, command string, args []string, cwd, prompt string, onText func(string)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.Command(command, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb", "FORCE_COLOR=0", "CLICOLOR=0")
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

	acpCtx, acpCancel := context.WithCancel(ctx)
	defer acpCancel()
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		acpCancel()
	}()
	go func() {
		<-acpCtx.Done()
		killProc(cmd)
	}()

	c := &acpClient{
		ctx:    acpCtx,
		w:      stdin,
		r:      bufio.NewReaderSize(stdout, 64<<10),
		wait:   map[string]chan rpc{},
		chunks: &strings.Builder{},
		onText: onText,
	}
	go c.readLoop()

	stop := func() error {
		acpCancel()
		_ = stdin.Close()
		killProc(cmd)
		select {
		case err := <-waitCh:
			return err
		case <-time.After(2 * time.Second):
			return fmt.Errorf("agent did not exit")
		}
	}

	if _, err := c.call("initialize", map[string]any{
		"protocolVersion": 1,
		"clientInfo":      map[string]string{"name": "leash", "title": "Leash", "version": version.String},
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
		"capabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
	}); err != nil {
		_ = stop()
		return "", fmt.Errorf("acp initialize: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	c.notify("initialized", map[string]any{})

	raw, err := c.call("session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})
	if err != nil {
		_ = stop()
		return "", fmt.Errorf("session/new: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(raw, &created)
	if created.SessionID == "" {
		_ = stop()
		return "", fmt.Errorf("session/new: no sessionId %s", string(raw))
	}

	_, promptErr := c.call("session/prompt", map[string]any{
		"sessionId": created.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": prompt}},
	})
	select {
	case <-time.After(250 * time.Millisecond):
	case <-acpCtx.Done():
	}
	reply := strings.TrimSpace(c.text())
	_ = stop()
	if reply != "" {
		return reply, nil
	}
	if promptErr != nil {
		return "", fmt.Errorf("session/prompt: %w %s", promptErr, strings.TrimSpace(stderr.String()))
	}
	return "", fmt.Errorf("acp: no agent message %s", strings.TrimSpace(stderr.String()))
}

func killProc(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

type acpClient struct {
	ctx    context.Context
	w      io.WriteCloser
	r      *bufio.Reader
	mu     sync.Mutex
	next   int
	wait   map[string]chan rpc
	chunks *strings.Builder
	onText func(string)
}

func (c *acpClient) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.chunks.String()
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

func (c *acpClient) failWait(err error) {
	raw := mustJSON(map[string]any{"code": -32000, "message": err.Error()})
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, ch := range c.wait {
		select {
		case ch <- rpc{Error: raw}:
		default:
		}
		delete(c.wait, k)
	}
}

func (c *acpClient) readLoop() {
	defer c.failWait(io.EOF)
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
	switch u.SessionUpdate {
	case "agent_message_chunk", "agent_message":
	default:
		return
	}
	if u.SessionUpdate == "agent_message" {
		c.mu.Lock()
		have := c.chunks.Len() > 0
		c.mu.Unlock()
		if have {
			return
		}
	}
	text := u.Text
	if text == "" {
		text = contentText(u.Content)
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	c.mu.Lock()
	c.chunks.WriteString(text)
	all := c.chunks.String()
	fn := c.onText
	c.mu.Unlock()
	if fn != nil {
		fn(clip(stripANSI(all)))
	}
}

func contentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var block struct {
		Text string `json:"text"`
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &block) == nil && strings.TrimSpace(block.Text) != "" {
		return block.Text
	}
	var blocks []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, x := range blocks {
			b.WriteString(x.Text)
		}
		return b.String()
	}
	return ""
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
