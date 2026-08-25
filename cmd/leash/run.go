package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/dispatch"
	"github.com/leashapp/leash/internal/server"
)

func cmdRun() error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	agent := fs.String("agent", "", "agent to start (claude, codex, opencode, cursor, hermes, grok)")
	fs.StringVar(agent, "a", "", "")
	cwd := fs.String("cwd", "", "project folder (default: current directory)")
	fs.StringVar(cwd, "C", "", "")
	fallback := fs.Bool("fallback", false, "if the named agent is missing, try the next installed one")
	list := fs.Bool("list", false, "list spawnable agents on this Mac")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: leash run [flags] [--] TASK

  leash run "fix the login"
  leash run --agent claude "fix the login"
  leash run --agent claude --fallback "fix the login"
  leash run --list

Starts one installed agent with a task. One job at a time. Does not fan out
onto the same working tree. Mission Control is the HUD; this is not a chat.

`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *list {
		return cmdRunList()
	}
	task := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if task == "" {
		fs.Usage()
		return fmt.Errorf("task required")
	}
	if strings.TrimSpace(*cwd) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		*cwd = wd
	}

	var job server.Job
	if err := rpc("POST", "/v1/run", map[string]any{
		"task":     task,
		"agent":    *agent,
		"cwd":      *cwd,
		"fallback": *fallback,
	}, &job); err != nil {
		return err
	}
	fmt.Printf("%s  %s\n", job.Agent, job.CWD)
	fmt.Printf("job %s  %s\n", job.ID, clipJobTitle(task))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			_ = rpc("POST", "/v1/interrupt", map[string]string{"text": "interrupted"}, nil)
			return fmt.Errorf("interrupted")
		case <-time.After(300 * time.Millisecond):
		}
		var st server.State
		if err := rpc("GET", "/v1/state", nil, &st); err != nil {
			return err
		}
		if st.Job == nil || st.Job.ID != job.ID {
			return fmt.Errorf("job disappeared")
		}
		switch st.Job.Status {
		case server.JobDone:
			fmt.Println("done")
			return nil
		case server.JobFailed:
			if st.Job.Error != "" {
				return fmt.Errorf("%s", st.Job.Error)
			}
			return fmt.Errorf("job failed")
		case server.JobKilled:
			return fmt.Errorf("killed")
		}
	}
}

func cmdRunList() error {
	found := agents.Scan(agents.DefaultProbe())
	n := 0
	for _, a := range found {
		if !a.Installed {
			fmt.Printf("  %-12s missing\n", a.Name)
			continue
		}
		if !dispatch.CanRun(a) {
			fmt.Printf("  %-12s installed  (not spawnable)\n", a.Name)
			continue
		}
		r, _ := dispatch.RecipeOf(a)
		fmt.Printf("  %-12s %-3s  %s\n", a.Name, r.Mode, a.Path)
		n++
	}
	if n == 0 {
		return fmt.Errorf("no spawnable agent on this Mac")
	}
	return nil
}

func clipJobTitle(task string) string {
	task = strings.TrimSpace(strings.ReplaceAll(task, "\n", " "))
	if len(task) <= 72 {
		return task
	}
	return task[:72] + "…"
}
