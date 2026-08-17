package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/leashapp/leash/internal/burst"
	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/policy"
)

type Server struct {
	Cfg        config.File
	Bursts     *burst.Store
	AskTimeout time.Duration
	Log        *slog.Logger
	Auto       func(policy.Assessment) hookfmt.Decision // tests only

	mu       sync.Mutex
	pending  map[string]*Pending
	lastKill time.Time
	recent   []recentDec
}

type Pending struct {
	ID         string    `json:"id"`
	Tool       string    `json:"tool"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail"`
	Kind       string    `json:"kind"`
	Reasons    []string  `json:"reasons"`
	Pattern    string    `json:"pattern"`
	CWD        string    `json:"cwd"`
	Created    time.Time `json:"created"`
	event      hookfmt.Event
	assessment policy.Assessment
	result     chan hookfmt.Decision
}

type State struct {
	Status    string   `json:"status"`
	WatchRoot string   `json:"watchRoot"`
	Pending   *Pending `json:"pending,omitempty"`
	Burst     *struct {
		ID        string    `json:"id"`
		Started   time.Time `json:"started"`
		FileCount int       `json:"fileCount"`
		Files     []string  `json:"files"`
	} `json:"burst,omitempty"`
	LastKill    *time.Time    `json:"lastKill,omitempty"`
	AlwaysAllow []policy.Rule `json:"alwaysAllow"`
	Port        int           `json:"port"`
}

func New(cfg config.File) *Server {
	return &Server{
		Cfg:        cfg,
		Bursts:     burst.NewStore(3 * time.Minute),
		AskTimeout: 8 * time.Minute,
		Log:        slog.Default(),
		pending:    map[string]*Pending{},
	}
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.Cfg.Port))
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/state", s.auth(s.handleState))
	mux.HandleFunc("POST /v1/hook", s.auth(s.handleHook))
	mux.HandleFunc("POST /v1/decision", s.auth(s.handleDecision))
	mux.HandleFunc("POST /v1/watch", s.auth(s.handleWatch))
	mux.HandleFunc("POST /v1/undo", s.auth(s.handleUndo))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	s.Log.Info("leash listening", "addr", ln.Addr().String())
	return srv.Serve(ln)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" {
			got = r.Header.Get("X-Leash-Token")
		}
		if s.Cfg.Token != "" && got != s.Cfg.Token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok":true}`)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Snapshot())
}

func (s *Server) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := State{
		Status:      "idle",
		WatchRoot:   s.Cfg.WatchRoot,
		AlwaysAllow: s.Cfg.AlwaysAllow,
		Port:        s.Cfg.Port,
	}
	if st.AlwaysAllow == nil {
		st.AlwaysAllow = []policy.Rule{}
	}
	if !s.lastKill.IsZero() {
		t := s.lastKill.UTC().Truncate(time.Second)
		st.LastKill = &t
	}
	for _, p := range s.pending {
		cp := *p
		st.Pending = &cp
		st.Status = "waiting"
		break
	}
	if b := s.Bursts.Last(); b != nil {
		st.Burst = &struct {
			ID        string    `json:"id"`
			Started   time.Time `json:"started"`
			FileCount int       `json:"fileCount"`
			Files     []string  `json:"files"`
		}{ID: b.ID, Started: b.Started.UTC().Truncate(time.Second), FileCount: b.FileCount, Files: b.Files()}
		if st.Status == "idle" {
			st.Status = "watching"
		}
	}
	return st
}

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	s.Cfg.WatchRoot = body.Path
	s.mu.Unlock()
	_ = config.Save(s.Cfg)
	writeJSON(w, s.Snapshot())
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	n, err := s.Bursts.Undo()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"restored": n})
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	d := hookfmt.Decision(body.Action)
	if d != hookfmt.DecisionAllow && d != hookfmt.DecisionAlways && d != hookfmt.DecisionKill {
		http.Error(w, "action must be allow, always, or kill", 400)
		return
	}
	if err := s.Resolve(body.ID, d); err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) Resolve(id string, d hookfmt.Decision) error {
	s.mu.Lock()
	p, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending approval")
	}
	if d == hookfmt.DecisionAlways {
		s.mu.Lock()
		s.Cfg.AlwaysAllow = append(s.Cfg.AlwaysAllow, policy.Rule{
			Tool:    p.Tool,
			Pattern: alwaysPattern(p),
		})
		cfg := s.Cfg
		s.mu.Unlock()
		_ = config.Save(cfg)
		d = hookfmt.DecisionAllow
	}
	if d == hookfmt.DecisionKill {
		s.mu.Lock()
		s.lastKill = time.Now()
		s.mu.Unlock()
	}
	select {
	case p.result <- d:
	default:
	}
	return nil
}

func alwaysPattern(p *Pending) string {
	if p.assessment.Pattern != "" {
		// store command/path only
		_, rest, ok := strings.Cut(p.assessment.Pattern, ":")
		if ok {
			return rest
		}
	}
	return p.Detail
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	out, err := s.HandleHook(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (s *Server) HandleHook(ctx context.Context, body []byte) ([]byte, error) {
	ev, err := hookfmt.Parse(body)
	if err != nil {
		return nil, err
	}
	if !hookfmt.IsPre(ev) {
		return hookfmt.SilentAllow(ev), nil
	}

	s.mu.Lock()
	watch := s.Cfg.WatchRoot
	always := append([]policy.Rule{}, s.Cfg.AlwaysAllow...)
	s.mu.Unlock()
	if watch == "" {
		watch = ev.CWD
	}

	a := policy.Assess(ev.ToolName, ev.CWD, watch, ev.ToolInput, always)
	root := watch
	if root == "" {
		root = ev.CWD
	}

	if a.Mutating && root != "" {
		b := s.Bursts.Begin(root, newID())
		b.Touch(a.Paths)
	}

	if a.Verdict == policy.Allow {
		return hookfmt.SilentAllow(ev), nil
	}

	if d, ok := s.recall(ev, a); ok {
		if d == hookfmt.DecisionKill {
			reason := "Blocked by Leash: " + strings.Join(a.Reasons, ", ")
			return hookfmt.Encode(ev, d, reason), nil
		}
		return hookfmt.Encode(ev, d, "Allowed by Leash"), nil
	}

	dec, err := s.ask(ctx, ev, a)
	if err != nil {
		return hookfmt.Encode(ev, hookfmt.DecisionKill, "Leash timed out"), nil
	}
	s.remember(ev, a, dec)
	reason := "Allowed by Leash"
	if dec == hookfmt.DecisionKill {
		reason = "Blocked by Leash: " + strings.Join(a.Reasons, ", ")
		if reason == "Blocked by Leash: " {
			reason = "Blocked by Leash"
		}
	}
	return hookfmt.Encode(ev, dec, reason), nil
}

func (s *Server) ask(ctx context.Context, ev hookfmt.Event, a policy.Assessment) (hookfmt.Decision, error) {
	if s.Auto != nil {
		return s.Auto(a), nil
	}
	p := &Pending{
		ID:         newID(),
		Tool:       policyTool(ev.ToolName),
		Title:      a.Title,
		Detail:     a.Detail,
		Kind:       a.Kind,
		Reasons:    a.Reasons,
		Pattern:    a.Pattern,
		CWD:        ev.CWD,
		Created:    time.Now(),
		event:      ev,
		assessment: a,
		result:     make(chan hookfmt.Decision, 1),
	}
	s.mu.Lock()
	s.pending[p.ID] = p
	s.mu.Unlock()

	timeout := s.AskTimeout
	if timeout <= 0 {
		timeout = 8 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case d := <-p.result:
		return d, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.pending, p.ID)
		s.mu.Unlock()
		return hookfmt.DecisionKill, ctx.Err()
	}
}

func policyTool(tool string) string {
	if tool == "" {
		return "Bash"
	}
	return tool
}

type recentDec struct {
	key string
	at  time.Time
	d   hookfmt.Decision
}

func decisionKey(ev hookfmt.Event, a policy.Assessment) string {
	return ev.CWD + "|" + a.Pattern
}

func (s *Server) remember(ev hookfmt.Event, a policy.Assessment, d hookfmt.Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, recentDec{key: decisionKey(ev, a), at: time.Now(), d: d})
	if len(s.recent) > 20 {
		s.recent = s.recent[len(s.recent)-20:]
	}
}

func (s *Server) recall(ev hookfmt.Event, a policy.Assessment) (hookfmt.Decision, bool) {
	key := decisionKey(ev, a)
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-3 * time.Second)
	for i := len(s.recent) - 1; i >= 0; i-- {
		r := s.recent[i]
		if r.at.Before(cutoff) {
			continue
		}
		if r.key == key {
			return r.d, true
		}
	}
	return "", false
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func ClientToken() (port int, token string, err error) {
	f, err := config.Load()
	if err != nil {
		return 0, "", err
	}
	return f.Port, f.Token, nil
}

func PostHook(port int, token string, body []byte, timeout time.Duration) ([]byte, int, error) {
	if timeout <= 0 {
		timeout = 9 * time.Minute
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/v1/hook", port), strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: timeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return data, res.StatusCode, nil
}

func DaemonRunning(port int) bool {
	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/health", port))
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == 200
}

func BinaryPath() (string, error) {
	return os.Executable()
}
