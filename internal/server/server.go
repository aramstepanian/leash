package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/burst"
	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/mission"
	"github.com/leashapp/leash/internal/policy"
)

const (
	maxHookBody = 1 << 20
	maxPending  = 32
	maxAlways   = 200
)

type Server struct {
	Cfg        config.File
	Bursts     *burst.Store
	AskTimeout time.Duration
	Log        *slog.Logger
	Auto       func(policy.Assessment) hookfmt.Decision // tests only

	mu        sync.Mutex
	pending   map[string]*Pending
	lastKill  time.Time
	recent    []recentDec
	http      *http.Server
	ready     chan struct{}
	readyOnce sync.Once
	mission   *mission.Log

	censusMu sync.Mutex
	census   []agents.Found
	censusAt time.Time

	job       *Job
	jobCancel context.CancelFunc
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
	Agent      string    `json:"agent,omitempty"`
	Root       string    `json:"root,omitempty"`
	Created    time.Time `json:"created"`
	event      hookfmt.Event
	assessment policy.Assessment
	result     chan hookfmt.Decision
}

type State struct {
	Status     string    `json:"status"`
	WatchRoot  string    `json:"watchRoot"`
	WatchRoots []string  `json:"watchRoots"`
	Pending    *Pending  `json:"pending,omitempty"`
	Queue      []Pending `json:"queue"`
	Waiting    int       `json:"waiting"`
	Burst      *struct {
		ID        string    `json:"id"`
		Started   time.Time `json:"started"`
		FileCount int       `json:"fileCount"`
		Files     []string  `json:"files"`
		Root      string    `json:"root,omitempty"`
	} `json:"burst,omitempty"`
	LastKill    *time.Time       `json:"lastKill,omitempty"`
	AlwaysAllow []policy.Rule    `json:"alwaysAllow"`
	Port        int              `json:"port"`
	Mission     mission.Snapshot `json:"mission"`
	Agents      []agents.Found   `json:"agents"`
	Job         *Job             `json:"job,omitempty"`
}

func New(cfg config.File) *Server {
	if len(cfg.WatchRoots) == 0 && cfg.WatchRoot != "" {
		cfg.WatchRoots = []string{cfg.WatchRoot}
	}
	if cfg.WatchRoots == nil {
		cfg.WatchRoots = []string{}
	}
	return &Server{
		Cfg:        cfg,
		Bursts:     burst.NewStore(3 * time.Minute),
		AskTimeout: 8 * time.Minute,
		Log:        slog.Default(),
		pending:    map[string]*Pending{},
		ready:      make(chan struct{}),
		mission:    &mission.Log{},
	}
}

func (s *Server) Ready() <-chan struct{} {
	if s.ready == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return s.ready
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.Cfg.Port))
	if err != nil {
		if isAddrInUse(err) {
			return fmt.Errorf("port %d already in use — is Leash already running?", s.Cfg.Port)
		}
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/state", s.auth(s.handleState))
	mux.HandleFunc("POST /v1/hook", s.auth(s.handleHook))
	mux.HandleFunc("POST /v1/decision", s.auth(s.handleDecision))
	mux.HandleFunc("POST /v1/watch", s.auth(s.handleWatch))
	mux.HandleFunc("POST /v1/undo", s.auth(s.handleUndo))
	mux.HandleFunc("POST /v1/steer", s.auth(s.handleSteer))
	mux.HandleFunc("POST /v1/interrupt", s.auth(s.handleInterrupt))
	mux.HandleFunc("POST /v1/retry", s.auth(s.handleRetry))
	mux.HandleFunc("POST /v1/skip", s.auth(s.handleSkip))
	mux.HandleFunc("POST /v1/always", s.auth(s.handleAlways))
	mux.HandleFunc("POST /v1/run", s.auth(s.handleRun))
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Minute,
		MaxHeaderBytes:    16 << 10,
	}
	s.mu.Lock()
	s.http = srv
	s.mu.Unlock()
	s.readyOnce.Do(func() {
		if s.ready != nil {
			close(s.ready)
		}
	})
	s.Log.Info("leash listening", "addr", ln.Addr().String())
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.http
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" {
			got = r.Header.Get("X-Leash-Token")
		}
		if !tokenOK(got, s.Cfg.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func tokenOK(got, want string) bool {
	if want == "" {
		return false
	}
	gb := []byte(got)
	wb := []byte(want)
	if len(gb) != len(wb) {
		_ = subtle.ConstantTimeCompare(wb, wb)
		return false
	}
	return subtle.ConstantTimeCompare(gb, wb) == 1
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
	roots := append([]string{}, s.Cfg.WatchRoots...)
	st := State{
		Status:      "idle",
		WatchRoot:   s.Cfg.WatchRoot,
		WatchRoots:  roots,
		Queue:       []Pending{},
		AlwaysAllow: s.Cfg.AlwaysAllow,
		Port:        s.Cfg.Port,
		Agents:      s.agentCensus(),
		Job:         s.snapshotJob(),
	}
	if len(st.WatchRoots) == 0 && st.WatchRoot != "" {
		st.WatchRoots = []string{st.WatchRoot}
	}
	if len(st.WatchRoots) > 0 && st.WatchRoot == "" {
		st.WatchRoot = st.WatchRoots[0]
	}
	if st.AlwaysAllow == nil {
		st.AlwaysAllow = []policy.Rule{}
	}
	if !s.lastKill.IsZero() {
		t := s.lastKill.UTC().Truncate(time.Second)
		st.LastKill = &t
	}
	oldest, rest := splitPending(s.pending)
	st.Waiting = len(s.pending)
	if oldest != nil {
		cp := *oldest
		st.Pending = &cp
		st.Queue = rest
		st.Status = "waiting"
	}
	if b := s.Bursts.Last(); b != nil {
		st.Burst = &struct {
			ID        string    `json:"id"`
			Started   time.Time `json:"started"`
			FileCount int       `json:"fileCount"`
			Files     []string  `json:"files"`
			Root      string    `json:"root,omitempty"`
		}{ID: b.ID, Started: b.Started.UTC().Truncate(time.Second), FileCount: b.FileCount, Files: b.Files(), Root: b.Root}
		if st.Status == "idle" {
			st.Status = "watching"
		}
	}
	waiting := st.Status == "waiting"
	st.Mission = s.mission.Snapshot(waiting, st.Burst != nil)
	st.Mission.Timeline = visibleTape(st.Mission.Timeline)
	if st.Mission.Phase == "act" && st.Status == "idle" {
		st.Status = "watching"
	}
	if st.Mission.Phase == "failed" {
		st.Status = "failed"
	}
	if st.Job != nil && st.Job.Active() && st.Status == "idle" {
		st.Status = "watching"
	}
	return st
}

func (s *Server) agentCensus() []agents.Found {
	s.censusMu.Lock()
	defer s.censusMu.Unlock()
	if s.census != nil && time.Since(s.censusAt) < 3*time.Second {
		return s.census
	}
	s.census = agents.Scan(agents.DefaultProbe())
	s.censusAt = time.Now()
	return s.census
}

func splitPending(m map[string]*Pending) (*Pending, []Pending) {
	var all []*Pending
	for _, p := range m {
		all = append(all, p)
	}
	if len(all) == 0 {
		return nil, []Pending{}
	}
	for i := 1; i < len(all); i++ {
		j := i
		for j > 0 && all[j].Created.Before(all[j-1].Created) {
			all[j], all[j-1] = all[j-1], all[j]
			j--
		}
	}
	oldest := all[0]
	rest := make([]Pending, 0, len(all)-1)
	for _, p := range all[1:] {
		cp := *p
		rest = append(rest, cp)
	}
	return oldest, rest
}

func oldestPending(m map[string]*Pending) *Pending {
	oldest, _ := splitPending(m)
	return oldest
}

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Remove bool   `json:"remove"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		http.Error(w, "watch path must be an existing directory", 400)
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, "watch path must be an existing directory", 400)
		return
	}
	if body.Remove {
		s.mu.Lock()
		s.Cfg.WatchRoots = policy.RemoveRoot(s.Cfg.WatchRoots, abs)
		if s.Cfg.WatchRoot != "" && policy.SameRoot(s.Cfg.WatchRoot, abs) {
			s.Cfg.WatchRoot = ""
		}
		if len(s.Cfg.WatchRoots) > 0 {
			s.Cfg.WatchRoot = s.Cfg.WatchRoots[0]
		}
		cfg := s.Cfg
		s.mu.Unlock()
		if err := config.Save(cfg); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, s.Snapshot())
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		http.Error(w, "watch path must be an existing directory", 400)
		return
	}
	s.mu.Lock()
	s.Cfg.WatchRoots = policy.AddRoot(s.Cfg.WatchRoots, abs)
	if len(s.Cfg.WatchRoots) > 0 {
		s.Cfg.WatchRoot = s.Cfg.WatchRoots[0]
	}
	cfg := s.Cfg
	s.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, s.Snapshot())
}

func (s *Server) handleAlways(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Remove  bool   `json:"remove"`
		Tool    string `json:"tool"`
		Pattern string `json:"pattern"`
		Root    string `json:"root"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !body.Remove {
		http.Error(w, "only remove is supported", 400)
		return
	}
	if strings.TrimSpace(body.Pattern) == "" {
		http.Error(w, "pattern required", 400)
		return
	}
	s.mu.Lock()
	s.Cfg.AlwaysAllow = policy.RemoveRule(s.Cfg.AlwaysAllow, body.Tool, body.Pattern, body.Root)
	cfg := s.Cfg
	s.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, s.Snapshot())
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	n, err := s.Bursts.Undo()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "undo",
		Title:  "Rewind",
		Detail: fmt.Sprintf("restored %d files", n),
		Result: "ok",
	})
	writeJSON(w, map[string]any{"restored": n})
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID     string `json:"id"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
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
			Root:    p.Root,
		})
		if len(s.Cfg.AlwaysAllow) > maxAlways {
			s.Cfg.AlwaysAllow = s.Cfg.AlwaysAllow[len(s.Cfg.AlwaysAllow)-maxAlways:]
		}
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
		_, rest, ok := strings.Cut(p.assessment.Pattern, ":")
		if ok {
			return rest
		}
	}
	return p.Detail
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxHookBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request too large or unreadable", 400)
		return
	}
	out, err := s.HandleHook(r.Context(), body)
	if err != nil {
		http.Error(w, "invalid hook payload", 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

var errTooManyPending = errors.New("too many pending approvals")

func (s *Server) HandleHook(ctx context.Context, body []byte) ([]byte, error) {
	ev, err := hookfmt.Parse(body)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	roots := append([]string{}, s.Cfg.WatchRoots...)
	if s.Cfg.WatchRoot != "" {
		roots = policy.AddRoot(roots, s.Cfg.WatchRoot)
	}
	always := append([]policy.Rule{}, s.Cfg.AlwaysAllow...)
	s.mu.Unlock()

	root := policy.MatchRoot(ev.CWD, roots)
	if root == "" {
		root = ev.CWD
	}
	if ev.CWD != "" && policy.ContainingRoot(ev.CWD, roots) == "" {
		s.rememberWatch(ev.CWD)
		root = policy.MatchRoot(ev.CWD, append(roots, ev.CWD))
	}

	if hookfmt.IsPlan(ev) {
		s.recordPlan(ev, root)
		return hookfmt.SilentAllow(ev), nil
	}
	if hookfmt.IsThought(ev) {
		s.recordThought(ev, root)
		return hookfmt.SilentAllow(ev), nil
	}
	if hookfmt.IsPost(ev) || hookfmt.IsFailure(ev) {
		return s.handlePostEvent(ev, root), nil
	}
	if !hookfmt.IsPre(ev) {
		return hookfmt.SilentAllow(ev), nil
	}

	s.Bursts.CloseIfIdle()

	a := policy.Assess(ev.ToolName, ev.CWD, root, ev.ToolInput, always)
	if len(a.Detail) > 8000 {
		a.Detail = a.Detail[:8000] + "…"
	}

	if a.Mutating && root != "" {
		b := s.Bursts.Begin(root, newID())
		b.Touch(a.Paths)
	}

	agent := hookfmt.AgentLabel(ev)

	if a.Verdict == policy.Allow {
		if policy.Quiet(ev.ToolName, a.Detail) {
			out := s.replyPre(ev, hookfmt.DecisionAllow, "Allowed by Leash")
			return out, nil
		}
		s.mission.StartLive(policyTool(ev.ToolName), a.Detail, agent, root, "running", a.Title)
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "tool",
			Agent:  agent,
			Tool:   policyTool(ev.ToolName),
			Title:  a.Title,
			Detail: a.Detail,
			Paths:  a.Paths,
			Root:   root,
			Result: "ok",
		})
		out := s.replyPre(ev, hookfmt.DecisionAllow, "Allowed by Leash")
		if !bytes.Contains(out, []byte("deny")) {
			return out, nil
		}
		return out, nil
	}

	if d, ok := s.recall(ev, a, root); ok {
		if d == hookfmt.DecisionKill {
			reason := "Blocked by Leash: " + strings.Join(a.Reasons, ", ")
			s.mission.Append(mission.Event{
				ID:     newID(),
				Kind:   "interrupt",
				Agent:  agent,
				Tool:   policyTool(ev.ToolName),
				Title:  a.Title,
				Detail: a.Detail,
				Root:   root,
				Result: "deny",
			})
			return s.replyPre(ev, d, reason), nil
		}
		s.mission.StartLive(policyTool(ev.ToolName), a.Detail, agent, root, "running", a.Title)
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "tool",
			Agent:  agent,
			Tool:   policyTool(ev.ToolName),
			Title:  a.Title,
			Detail: a.Detail,
			Paths:  a.Paths,
			Root:   root,
			Result: "ok",
		})
		return s.replyPre(ev, d, "Allowed by Leash"), nil
	}

	s.mission.StartLive(policyTool(ev.ToolName), a.Detail, agent, root, "waiting", a.Title)
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "gate",
		Agent:  agent,
		Tool:   policyTool(ev.ToolName),
		Title:  a.Title,
		Detail: a.Detail,
		Paths:  a.Paths,
		Root:   root,
		Result: "waiting",
	})

	dec, err := s.ask(ctx, ev, a, root)
	if err != nil {
		reason := "Leash timed out"
		if errors.Is(err, errTooManyPending) {
			reason = "Leash is busy"
		}
		s.mission.ClearLive()
		return s.replyPre(ev, hookfmt.DecisionKill, reason), nil
	}
	s.remember(ev, a, dec, root)
	reason := "Allowed by Leash"
	if dec == hookfmt.DecisionKill {
		reason = "Blocked by Leash: " + strings.Join(a.Reasons, ", ")
		if reason == "Blocked by Leash: " {
			reason = "Blocked by Leash"
		}
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "interrupt",
			Agent:  agent,
			Tool:   policyTool(ev.ToolName),
			Title:  a.Title,
			Detail: a.Detail,
			Root:   root,
			Result: "deny",
		})
		s.mission.ClearLive()
	} else {
		s.mission.StartLive(policyTool(ev.ToolName), a.Detail, agent, root, "running", a.Title)
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "tool",
			Agent:  agent,
			Tool:   policyTool(ev.ToolName),
			Title:  a.Title,
			Detail: a.Detail,
			Paths:  a.Paths,
			Root:   root,
			Result: "ok",
		})
	}
	return s.replyPre(ev, dec, reason), nil
}

func (s *Server) rememberWatch(cwd string) {
	s.mu.Lock()
	before := len(s.Cfg.WatchRoots)
	s.Cfg.WatchRoots = policy.RememberRoot(s.Cfg.WatchRoots, cwd)
	if len(s.Cfg.WatchRoots) == before {
		s.mu.Unlock()
		return
	}
	if len(s.Cfg.WatchRoots) > 0 && s.Cfg.WatchRoot == "" {
		s.Cfg.WatchRoot = s.Cfg.WatchRoots[0]
	}
	cfg := s.Cfg
	s.mu.Unlock()
	_ = config.Save(cfg)
}

func (s *Server) ask(ctx context.Context, ev hookfmt.Event, a policy.Assessment, root string) (hookfmt.Decision, error) {
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
		Agent:      hookfmt.AgentLabel(ev),
		Root:       root,
		Created:    time.Now(),
		event:      ev,
		assessment: a,
		result:     make(chan hookfmt.Decision, 1),
	}
	s.mu.Lock()
	if len(s.pending) >= maxPending {
		s.mu.Unlock()
		return hookfmt.DecisionKill, errTooManyPending
	}
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

func decisionKey(ev hookfmt.Event, a policy.Assessment, root string) string {
	return root + "|" + hookfmt.AgentLabel(ev) + "|" + a.Pattern
}

func (s *Server) remember(ev hookfmt.Event, a policy.Assessment, d hookfmt.Decision, root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, recentDec{key: decisionKey(ev, a, root), at: time.Now(), d: d})
	if len(s.recent) > 20 {
		s.recent = s.recent[len(s.recent)-20:]
	}
}

func (s *Server) recall(ev hookfmt.Event, a policy.Assessment, root string) (hookfmt.Decision, bool) {
	key := decisionKey(ev, a, root)
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

func PostHook(port int, token string, body []byte, timeout time.Duration) ([]byte, int, error) {
	if timeout <= 0 {
		timeout = 9 * time.Minute
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/v1/hook", port), bytes.NewReader(body))
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
	data, _ := io.ReadAll(io.LimitReader(res.Body, maxHookBody))
	return data, res.StatusCode, nil
}

func DaemonRunning(port int) bool {
	client := &http.Client{Timeout: 400 * time.Millisecond}
	res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/v1/health", port))
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == 200
}
