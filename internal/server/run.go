package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/dispatch"
)

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt string `json:"prompt"`
		Agent  string `json:"agent"`
		Path   string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32<<10)).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	prompt := strings.TrimSpace(body.Prompt)
	if prompt == "" {
		http.Error(w, "prompt required", 400)
		return
	}

	s.mu.Lock()
	if s.job != nil && s.job.Status == "running" {
		s.mu.Unlock()
		http.Error(w, "already running", http.StatusConflict)
		return
	}
	root := strings.TrimSpace(body.Path)
	if root == "" {
		root = s.Cfg.WatchRoot
	}
	if root == "" && len(s.Cfg.WatchRoots) > 0 {
		root = s.Cfg.WatchRoots[0]
	}
	s.mu.Unlock()

	if root == "" {
		http.Error(w, "pick a folder", 400)
		return
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		http.Error(w, "pick a folder", 400)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		http.Error(w, "path must be an existing directory", 400)
		return
	}

	s.mu.Lock()
	runner := s.RunJob
	busy := s.job != nil && s.job.Status == "running"
	s.mu.Unlock()
	if busy {
		http.Error(w, "already running", http.StatusConflict)
		return
	}
	if runner == nil {
		if _, err := dispatch.Pick(agents.DefaultProbe(), strings.TrimSpace(body.Agent)); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job := dispatch.Job{Prompt: prompt, Agent: strings.TrimSpace(body.Agent), Root: abs}

	s.mu.Lock()
	if s.job != nil && s.job.Status == "running" {
		s.mu.Unlock()
		cancel()
		http.Error(w, "already running", http.StatusConflict)
		return
	}
	if s.jobCancel != nil {
		s.jobCancel()
	}
	s.jobCancel = cancel
	s.job = &Job{Prompt: prompt, Agent: job.Agent, Root: abs, Status: "running"}
	s.mu.Unlock()

	go s.execJob(ctx, job, runner)
	writeJSON(w, s.Snapshot())
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.jobCancel != nil {
		s.jobCancel()
	}
	s.mu.Unlock()
	writeJSON(w, s.Snapshot())
}

func (s *Server) execJob(ctx context.Context, job dispatch.Job, runner func(context.Context, dispatch.Job) (string, string, error)) {
	if runner == nil {
		runner = dispatch.Run
	}
	name, result, err := runner(ctx, job)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil || s.job.Prompt != job.Prompt || s.job.Root != job.Root {
		return
	}
	if name != "" {
		s.job.Agent = name
	}
	if err != nil {
		s.job.Status = "failed"
		s.job.Error = err.Error()
		s.job.Result = result
	} else {
		s.job.Status = "done"
		s.job.Error = ""
		s.job.Result = result
	}
	if s.jobCancel != nil {
		s.jobCancel()
		s.jobCancel = nil
	}
}
