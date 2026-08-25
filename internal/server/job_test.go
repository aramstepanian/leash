package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leashapp/leash/internal/config"
)

func TestStartJobRunsFakeAgent(t *testing.T) {
	home, bin, root := jobEnv(t)
	writeAgent(t, filepath.Join(bin, "claude"), "#!/bin/sh\nexit 0\n")
	s := New(config.File{Token: "t", WatchRoots: []string{root}})
	job, err := s.StartJob(JobRequest{Task: "fix the login", Agent: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	got := waitJob(t, s, JobDone)
	if got.ID != job.ID || got.Agent != "Claude" || got.CWD != root {
		t.Fatalf("%+v", got)
	}
	st := s.Snapshot()
	if st.Mission.Title != "fix the login" {
		t.Fatalf("title %q", st.Mission.Title)
	}
	if st.Mission.Agent != "Claude" {
		t.Fatalf("agent %q", st.Mission.Agent)
	}
	_ = home
}

func TestStartJobBusyAndInterrupt(t *testing.T) {
	_, bin, root := jobEnv(t)
	writeAgent(t, filepath.Join(bin, "claude"), "#!/bin/sh\ntrap 'exit 0' TERM\nwhile true; do sleep 0.05; done\n")
	s := New(config.File{Token: "t", WatchRoots: []string{root}})
	if _, err := s.StartJob(JobRequest{Task: "hang", Agent: "claude"}); err != nil {
		t.Fatal(err)
	}
	waitJob(t, s, JobRunning)
	if _, err := s.StartJob(JobRequest{Task: "second", Agent: "claude"}); err != errJobBusy {
		t.Fatalf("want busy, got %v", err)
	}
	if !s.StopJob("cut") {
		t.Fatal("expected stop")
	}
	got := waitJob(t, s, JobKilled)
	if got.Error == "" {
		t.Fatalf("%+v", got)
	}
}

func TestStartJobFallback(t *testing.T) {
	_, bin, root := jobEnv(t)
	writeAgent(t, filepath.Join(bin, "codex"), "#!/bin/sh\nexit 0\n")
	s := New(config.File{Token: "t", WatchRoots: []string{root}})
	if _, err := s.StartJob(JobRequest{Task: "fix", Agent: "claude"}); err == nil {
		t.Fatal("expected missing claude")
	}
	job, err := s.StartJob(JobRequest{Task: "fix", Agent: "claude", Fallback: true})
	if err != nil {
		t.Fatal(err)
	}
	got := waitJob(t, s, JobDone)
	if got.ID != job.ID {
		t.Fatalf("%+v", got)
	}
	if got.Agent != "Codex" {
		t.Fatalf("fallback agent %q", got.Agent)
	}
	if got.FallbackFrom == "" && got.AgentID != "codex" {
		t.Fatalf("want codex fallback %+v", got)
	}
}

func TestStartJobRequiresCWD(t *testing.T) {
	s := New(config.File{Token: "t"})
	if _, err := s.StartJob(JobRequest{Task: "fix"}); err == nil {
		t.Fatal("expected cwd error")
	}
}

func jobEnv(t *testing.T) (home, bin, root string) {
	t.Helper()
	home = t.TempDir()
	bin = filepath.Join(home, "bin")
	root = t.TempDir()
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEASH_HOME", home)
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	return home, bin, root
}

func writeAgent(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func waitJob(t *testing.T, s *Server, want string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last *Job
	for time.Now().Before(deadline) {
		st := s.Snapshot()
		last = st.Job
		if st.Job != nil && st.Job.Status == want {
			return *st.Job
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("job not %s: %+v", want, last)
	return Job{}
}
