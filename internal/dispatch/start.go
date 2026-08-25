package dispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/leashapp/leash/internal/acp"
)

// StartError means the binary could not be launched (missing, not executable).
// A later non-zero exit is a normal job failure, not a StartError.
type StartError struct {
	Err error
}

func (e *StartError) Error() string {
	if e == nil || e.Err == nil {
		return "could not start agent"
	}
	return e.Err.Error()
}

func (e *StartError) Unwrap() error { return e.Err }

func IsStartError(err error) bool {
	var s *StartError
	return errors.As(err, &s)
}

// Start runs one recipe until the agent exits or ctx is cancelled.
// log receives CLI stdout/stderr (and ACP stderr). It may be nil.
func Start(ctx context.Context, r Recipe, cwd, prompt string, gate acp.Gate, notify acp.Notify, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	if r.Mode == ModeACP {
		return startACP(ctx, r, cwd, prompt, gate, notify, log)
	}
	return startCLI(ctx, r, cwd, prompt, log)
}

func startCLI(ctx context.Context, r Recipe, cwd, prompt string, log io.Writer) error {
	cmd := exec.Command(r.Command, r.Argv(prompt)...)
	prepare(cmd, cwd, r.Name, log, log)
	if err := cmd.Start(); err != nil {
		return &StartError{Err: err}
	}
	return waitCtx(ctx, cmd)
}

func startACP(ctx context.Context, r Recipe, cwd, prompt string, gate acp.Gate, notify acp.Notify, log io.Writer) error {
	cmd := exec.Command(r.Command, r.Args...)
	prepare(cmd, cwd, r.Name, nil, log)
	agentIn, err := cmd.StdinPipe()
	if err != nil {
		return &StartError{Err: err}
	}
	agentOut, err := cmd.StdoutPipe()
	if err != nil {
		return &StartError{Err: err}
	}
	if err := cmd.Start(); err != nil {
		return &StartError{Err: err}
	}

	protoDone := make(chan error, 1)
	go func() {
		err := acp.ServePrompt(ctx, agentIn, agentOut, r.Name, cwd, prompt, gate, notify)
		_ = agentIn.Close()
		protoDone <- err
	}()

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		killGroup(cmd)
		<-waitDone
		<-protoDone
		return ctx.Err()
	case err := <-protoDone:
		if ctx.Err() != nil {
			killGroup(cmd)
			<-waitDone
			return ctx.Err()
		}
		killGroup(cmd)
		waitErr := <-waitDone
		if err != nil {
			return err
		}
		return waitErr
	case err := <-waitDone:
		select {
		case protoErr := <-protoDone:
			if protoErr != nil {
				return protoErr
			}
		default:
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
}

func prepare(cmd *exec.Cmd, cwd, agent string, stdout, stderr io.Writer) {
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = append(os.Environ(), "LEASH_AGENT="+agent)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}
}

func waitCtx(ctx context.Context, cmd *exec.Cmd) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		killGroup(cmd)
		<-done
		return ctx.Err()
	case err := <-done:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
}

func killGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
}
