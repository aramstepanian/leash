package dispatch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/leashapp/leash/internal/agents"
)

const maxResult = 8000

// Job is one prompt sent to one agent.
type Job struct {
	Prompt string
	Agent  string
	Root   string
}

// Run sends prompt to an installed CLI agent and waits until it exits.
func Run(ctx context.Context, job Job) (agentName, result string, err error) {
	return RunWith(ctx, job, agents.DefaultProbe())
}

// RunWith is Run with an explicit probe (tests).
func RunWith(ctx context.Context, job Job, p agents.Probe) (string, string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
	}
	found, err := Pick(p, job.Agent)
	if err != nil {
		return "", "", err
	}
	rec, err := For(found, job.Prompt)
	if err != nil {
		return "", "", err
	}
	root := strings.TrimSpace(job.Root)
	if root == "" {
		root, _ = os.Getwd()
	}
	if rec.ACP {
		out, err := oneShotACP(ctx, rec.Command, rec.Args, root, job.Prompt)
		return found.Name, clip(out), err
	}
	out, err := runPrint(ctx, rec, root)
	if err != nil && found.ACP != "" {
		args := strings.Fields(found.ACP)
		if len(args) > 1 {
			out2, err2 := oneShotACP(ctx, rec.Command, args[1:], root, job.Prompt)
			if err2 == nil {
				return found.Name, clip(out2), nil
			}
		}
	}
	return found.Name, clip(out), err
}

func runPrint(ctx context.Context, rec Recipe, root string) (string, error) {
	if rec.Command == "" {
		return "", fmt.Errorf("no command")
	}
	cmd := exec.CommandContext(ctx, rec.Command, rec.Args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb", "FORCE_COLOR=0", "CLICOLOR=0")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%s: %w\n%s", rec.Name, err, clip(buf.String()))
	}
	return buf.String(), nil
}

var (
	ansiCSI    = regexp.MustCompile(`\x1b\[[0-9;?=]*[ -/]*[@-~]`)
	ansiOSC    = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	ansiOther  = regexp.MustCompile(`\x1b[@-Z\\-_]`)
	ansiOrphan = regexp.MustCompile(`\[[0-9;]{1,12}m`)
)

func clip(s string) string {
	s = stripANSI(s)
	s = strings.TrimSpace(s)
	if len(s) <= maxResult {
		return s
	}
	return s[:maxResult] + "…"
}

func stripANSI(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiOther.ReplaceAllString(s, "")
	s = ansiOrphan.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\x1b", "")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
