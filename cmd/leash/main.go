package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/install"
	"github.com/leashapp/leash/internal/server"
)

const (
	version     = "0.3.0"
	maxHookBody = 1 << 20
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe()
	case "hook":
		err = cmdHook()
	case "undo":
		err = cmdUndo()
	case "install":
		err = cmdInstall()
	case "uninstall":
		err = cmdUninstall()
	case "status":
		err = cmdStatus()
	case "watch":
		err = cmdWatch()
	case "demo":
		err = cmdDemo()
	case "decide":
		err = cmdDecide()
	case "version", "-v", "--version":
		fmt.Println(version)
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "leash: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Leash — seatbelt for coding agents

  leash serve              Start the local daemon (127.0.0.1)
  leash hook               Called by any hooked agent (reads stdin JSON)
  leash install            Wire Cursor, Claude Code, Codex, OpenCode
  leash uninstall          Remove those hooks / plugin
  leash watch [path]       Folder to protect (default: cwd)
  leash undo               Restore files from the last agent burst
  leash status             Show daemon + pending approval
  leash demo [command]     Fake a dangerous hook (for recording)
  leash decide ID allow|always|kill
  leash version
`)
}

func cmdServe() error {
	cfg, err := config.Ensure()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	s := server.New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-s.Ready():
	}

	select {
	case err := <-errCh:
		return err
	case sig := <-sigs:
		slog.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			return err
		}
		return <-errCh
	}
}

func cmdHook() error {
	in, err := io.ReadAll(io.LimitReader(os.Stdin, maxHookBody+1))
	if err != nil {
		return failOpen("could not read hook stdin, allowing tool")
	}
	if len(in) > maxHookBody {
		return failOpen("hook payload too large, allowing tool")
	}
	cfg, err := config.Load()
	if err != nil {
		return failOpen("daemon unavailable, allowing tool")
	}
	if cfg.Token == "" || !server.DaemonRunning(cfg.Port) {
		return failOpen("daemon unavailable, allowing tool")
	}
	out, code, err := server.PostHook(cfg.Port, cfg.Token, in, 9*time.Minute)
	if err != nil {
		return failOpen("daemon unavailable, allowing tool")
	}
	if code != 200 {
		return failOpen("daemon unavailable, allowing tool")
	}
	os.Stdout.Write(out)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

func failOpen(msg string) error {
	fmt.Fprintln(os.Stderr, "leash:", msg)
	fmt.Fprint(os.Stdout, "{}\n")
	return nil
}

func cmdInstall() error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	bin, err = filepath.EvalSymlinks(bin)
	if err != nil {
		bin, _ = os.Executable()
	}
	if err := install.Install(bin, 540); err != nil {
		return err
	}
	fmt.Println("hooks installed")
	fmt.Println("  Cursor       ~/.cursor/hooks.json")
	fmt.Println("  Claude Code  ~/.claude/settings.json")
	fmt.Println("  Codex        ~/.codex/hooks.json")
	fmt.Println("  OpenCode     ~/.config/opencode/plugins/leash.js")
	fmt.Println("then: leash serve   (or open Leash.app)")
	fmt.Println("custom agent: see docs/INTEGRATION.md")
	return nil
}

func cmdUninstall() error {
	if err := install.Uninstall(); err != nil {
		return err
	}
	fmt.Println("hooks removed")
	return nil
}

func cmdWatch() error {
	path := ""
	if len(os.Args) > 2 {
		path = os.Args[2]
	} else {
		path, _ = os.Getwd()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return rpc("POST", "/v1/watch", map[string]string{"path": abs}, nil)
}

func cmdUndo() error {
	var out map[string]any
	if err := rpc("POST", "/v1/undo", map[string]any{}, &out); err != nil {
		return err
	}
	fmt.Printf("restored %v files\n", out["restored"])
	return nil
}

func cmdStatus() error {
	var st server.State
	if err := rpc("GET", "/v1/state", nil, &st); err != nil {
		cfg, _ := config.Load()
		if !server.DaemonRunning(cfg.Port) {
			fmt.Println("daemon: off")
			fmt.Println("start with: leash serve")
			return nil
		}
		return err
	}
	fmt.Printf("daemon: on  :%d  %s\n", st.Port, st.Status)
	if st.WatchRoot != "" {
		fmt.Printf("watch:  %s\n", st.WatchRoot)
	}
	if st.Pending != nil {
		fmt.Printf("pending %s  %s\n%s\n", st.Pending.ID, st.Pending.Title, st.Pending.Detail)
		fmt.Println("decide: leash decide", st.Pending.ID, "allow|always|kill")
	}
	if st.Burst != nil {
		fmt.Printf("burst:  %d files\n", st.Burst.FileCount)
	}
	return nil
}

func cmdDecide() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("usage: leash decide ID allow|always|kill")
	}
	return rpc("POST", "/v1/decision", map[string]string{
		"id":     os.Args[2],
		"action": os.Args[3],
	}, nil)
}

func cmdDemo() error {
	cmd := "rm -rf ./dist"
	if len(os.Args) > 2 {
		cmd = strings.Join(os.Args[2:], " ")
	}
	cwd, _ := os.Getwd()
	body, _ := json.Marshal(map[string]any{
		"session_id":      "demo",
		"cwd":             cwd,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]string{"command": cmd},
	})
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !server.DaemonRunning(cfg.Port) {
		return fmt.Errorf("daemon not running — start Leash.app or `leash serve`")
	}
	fmt.Fprintf(os.Stderr, "demo: %s\n", cmd)
	out, code, err := server.PostHook(cfg.Port, cfg.Token, body, 9*time.Minute)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("http %d: %s", code, out)
	}
	fmt.Println(string(bytes.TrimSpace(out)))
	return nil
}

func rpc(method, path string, body any, dest any) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var rdr io.Reader
	if body != nil && method != "GET" {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Port, path), rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("daemon not running (%w)", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, maxHookBody))
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s", strings.TrimSpace(string(data)))
	}
	if dest != nil && len(data) > 0 {
		return json.Unmarshal(data, dest)
	}
	if dest == nil && len(data) > 0 {
		fmt.Println(strings.TrimSpace(string(data)))
	}
	return nil
}
