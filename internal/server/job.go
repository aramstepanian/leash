package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leashapp/leash/internal/acp"
	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/dispatch"
	"github.com/leashapp/leash/internal/hookfmt"
	"github.com/leashapp/leash/internal/mission"
)

const (
	JobStarting = "starting"
	JobRunning  = "running"
	JobDone     = "done"
	JobFailed   = "failed"
	JobKilled   = "killed"
)

var errJobBusy = errors.New("a job is already running")

type Job struct {
	ID           string     `json:"id"`
	Task         string     `json:"task"`
	Agent        string     `json:"agent"`
	AgentID      string     `json:"agentId"`
	CWD          string     `json:"cwd"`
	Status       string     `json:"status"`
	Error        string     `json:"error,omitempty"`
	Started      time.Time  `json:"started"`
	Ended        *time.Time `json:"ended,omitempty"`
	FallbackFrom string     `json:"fallbackFrom,omitempty"`
	Want         string     `json:"-"`
	Fallback     bool       `json:"-"`
}

func (j Job) Active() bool {
	return j.Status == JobStarting || j.Status == JobRunning
}

type JobRequest struct {
	Task     string `json:"task"`
	Agent    string `json:"agent"`
	CWD      string `json:"cwd"`
	Fallback bool   `json:"fallback"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	job, err := s.StartJob(req)
	if err != nil {
		code := 400
		if errors.Is(err, errJobBusy) {
			code = http.StatusConflict
		}
		http.Error(w, err.Error(), code)
		return
	}
	writeJSON(w, job)
}

func (s *Server) StartJob(req JobRequest) (*Job, error) {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, fmt.Errorf("task required")
	}
	cwd, err := s.resolveJobCWD(req.CWD)
	if err != nil {
		return nil, err
	}
	found := s.agentCensus()
	cands, err := dispatch.Candidates(req.Agent, req.Fallback, found)
	if err != nil {
		return nil, err
	}
	first := cands[0]
	recipe, err := dispatch.RecipeOf(first)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if s.job != nil && s.job.Active() {
		s.mu.Unlock()
		return nil, errJobBusy
	}
	now := time.Now().UTC().Truncate(time.Second)
	job := &Job{
		ID:       newID(),
		Task:     task,
		Agent:    recipe.Name,
		AgentID:  recipe.ID,
		CWD:      cwd,
		Status:   JobStarting,
		Started:  now,
		Want:     req.Agent,
		Fallback: req.Fallback,
	}
	s.job = job
	s.mu.Unlock()

	s.rememberWatch(cwd)
	s.mission.Reset()
	title := clipJobTitle(task)
	s.mission.Append(mission.Event{
		ID:     newID(),
		Kind:   "plan",
		Agent:  recipe.Name,
		Title:  title,
		Detail: task,
		Root:   cwd,
		Result: "ok",
	})
	s.mission.StartLive("Job", task, recipe.Name, cwd, "running", title)

	go s.runCandidates(*job, cands, task, cwd)
	cp := *job
	return &cp, nil
}

func (s *Server) StopJob(reason string) bool {
	s.mu.Lock()
	if s.job == nil || !s.job.Active() {
		s.mu.Unlock()
		return false
	}
	s.job.Status = JobKilled
	s.job.Error = firstNonEmpty(reason, "interrupted")
	now := time.Now().UTC().Truncate(time.Second)
	s.job.Ended = &now
	cancel := s.jobCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.mission.ClearLive()
	return true
}

func (s *Server) runCandidates(seed Job, cands []agents.Found, task, cwd string) {
	s.mu.Lock()
	if s.job == nil || s.job.ID != seed.ID {
		s.mu.Unlock()
		return
	}
	s.job.Status = JobRunning
	s.mu.Unlock()

	var fallbackFrom string
	for i, f := range cands {
		recipe, err := dispatch.RecipeOf(f)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		if s.job == nil || s.job.ID != seed.ID || s.job.Status == JobKilled {
			s.mu.Unlock()
			cancel()
			return
		}
		if i > 0 && fallbackFrom == "" {
			s.job.FallbackFrom = s.job.Agent
			fallbackFrom = s.job.FallbackFrom
		}
		s.job.Agent = recipe.Name
		s.job.AgentID = recipe.ID
		s.jobCancel = cancel
		s.mu.Unlock()

		logw := jobLog(seed.ID)
		err = dispatch.Start(ctx, recipe, cwd, task, s.hookGate(), s.hookNotify(), logw)
		if closer, ok := logw.(io.Closer); ok {
			_ = closer.Close()
		}
		if s.jobWasKilled(seed.ID) {
			return
		}
		if err == nil {
			s.finishJob(seed.ID, JobDone, "")
			return
		}
		if errors.Is(err, context.Canceled) {
			s.finishJob(seed.ID, JobKilled, "interrupted")
			return
		}
		if dispatch.IsStartError(err) && i+1 < len(cands) {
			continue
		}
		s.finishJob(seed.ID, JobFailed, err.Error())
		return
	}
	s.finishJob(seed.ID, JobFailed, "no spawnable agent could be started")
}

func (s *Server) jobWasKilled(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.job != nil && s.job.ID == id && s.job.Status == JobKilled
}

func (s *Server) finishJob(id, status, errText string) {
	s.mu.Lock()
	if s.job == nil || s.job.ID != id {
		s.mu.Unlock()
		return
	}
	if s.job.Status == JobKilled || s.job.Status == JobDone || s.job.Status == JobFailed {
		s.mu.Unlock()
		return
	}
	s.job.Status = status
	s.job.Error = errText
	now := time.Now().UTC().Truncate(time.Second)
	s.job.Ended = &now
	cancel := s.jobCancel
	s.jobCancel = nil
	agent := s.job.Agent
	task := s.job.Task
	cwd := s.job.CWD
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	s.mission.ClearLive()
	switch status {
	case JobFailed:
		s.mission.MarkFailed(mission.Failed{Tool: "Job", Detail: task, Outcome: clipJobTitle(task), Error: errText, Agent: agent})
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "error",
			Agent:  agent,
			Title:  clipJobTitle(task),
			Detail: errText,
			Root:   cwd,
			Result: "error",
			Error:  errText,
		})
	case JobKilled:
		s.mission.Append(mission.Event{
			ID:     newID(),
			Kind:   "interrupt",
			Agent:  agent,
			Title:  "Interrupt",
			Detail: firstNonEmpty(errText, "interrupted"),
			Root:   cwd,
			Result: "deny",
		})
	}
}

func (s *Server) resolveJobCWD(cwd string) (string, error) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		s.mu.Lock()
		roots := append([]string{}, s.Cfg.WatchRoots...)
		if s.Cfg.WatchRoot != "" {
			roots = append([]string{s.Cfg.WatchRoot}, roots...)
		}
		s.mu.Unlock()
		if len(roots) == 0 {
			return "", fmt.Errorf("cwd required — watch a folder or pass --cwd")
		}
		cwd = roots[0]
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("cwd must be an existing directory")
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("cwd must be an existing directory")
	}
	return abs, nil
}

func (s *Server) snapshotJob() *Job {
	if s.job == nil {
		return nil
	}
	cp := *s.job
	return &cp
}

func (s *Server) hookGate() acp.Gate {
	return func(ctx context.Context, ev hookfmt.Event) hookfmt.Decision {
		body, err := json.Marshal(map[string]any{
			"protocol":        "leash",
			"hook_event_name": "pre_tool",
			"agent":           ev.Agent,
			"cwd":             ev.CWD,
			"tool_name":       ev.ToolName,
			"tool_input":      ev.ToolInput,
			"text":            ev.Text,
		})
		if err != nil {
			return hookfmt.DecisionAllow
		}
		out, err := s.HandleHook(ctx, body)
		if err != nil {
			return hookfmt.DecisionAllow
		}
		var parsed struct {
			Decision string `json:"decision"`
		}
		if json.Unmarshal(out, &parsed) != nil {
			return hookfmt.DecisionAllow
		}
		if parsed.Decision == "deny" {
			return hookfmt.DecisionKill
		}
		return hookfmt.DecisionAllow
	}
}

func (s *Server) hookNotify() acp.Notify {
	return func(ev hookfmt.Event) {
		body, err := json.Marshal(map[string]any{
			"protocol":        "leash",
			"hook_event_name": ev.HookEventName,
			"agent":           ev.Agent,
			"cwd":             ev.CWD,
			"tool_name":       ev.ToolName,
			"tool_input":      ev.ToolInput,
			"text":            ev.Text,
			"steps":           ev.Steps,
		})
		if err != nil {
			return
		}
		_, _ = s.HandleHook(context.Background(), body)
	}
}

func jobLog(id string) io.WriteCloser {
	dir := filepath.Join(config.Dir(), "jobs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nopWriteCloser{Writer: io.Discard}
	}
	f, err := os.OpenFile(filepath.Join(dir, id+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nopWriteCloser{Writer: io.Discard}
	}
	return f
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func clipJobTitle(task string) string {
	task = strings.TrimSpace(strings.ReplaceAll(task, "\n", " "))
	if len(task) <= 72 {
		return task
	}
	return task[:72] + "…"
}
