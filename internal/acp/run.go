package acp

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
)

// Run spawns the agent and proxies ACP stdio until either side closes.
func Run(ctx context.Context, launch Launch, clientIn io.Reader, clientOut, errOut io.Writer, gate Gate, notify Notify) error {
	cmd := exec.CommandContext(ctx, launch.Command, launch.Args...)
	cmd.Stderr = errOut
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	agentIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	agentOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- Proxy(ctx, clientIn, clientOut, agentIn, agentOut, launch.Name, gate, notify)
		_ = agentIn.Close()
	}()

	waitErr := cmd.Wait()
	select {
	case proxyErr := <-done:
		if waitErr != nil {
			return waitErr
		}
		return proxyErr
	default:
		return waitErr
	}
}

func RunStdio(ctx context.Context, launch Launch, gate Gate, notify Notify) error {
	return Run(ctx, launch, os.Stdin, os.Stdout, os.Stderr, gate, notify)
}
