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

	"github.com/leashapp/leash/internal/acp"
	"github.com/leashapp/leash/internal/agents"
	"github.com/leashapp/leash/internal/config"
	"github.com/leashapp/leash/internal/install"
	"github.com/leashapp/leash/internal/server"
)

const (
	version     = "0.8.0"
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
	case "run":
		err = cmdRun()
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
	case "steer":
		err = cmdSteer()
	case "interrupt":
		err = cmdInterrupt()
	case "retry":
		err = cmdRetry()
	case "skip":
		err = cmdSkip()
	case "always":
		err = cmdAlways()
	case "acp":
		err = cmdACP()
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
	fmt.Fprint(os.Stderr, `Leash — seatbelt + mission control for coding agents

  leash serve              Start the local daemon (127.0.0.1)
  leash run TASK           Start one installed agent with a job
  leash run --list         Show which agents Leash can spawn
  leash hook               Called by any hooked agent (reads stdin JSON)
  leash install            Wire Cursor, Claude Code, Codex, OpenCode
  leash uninstall          Remove those hooks / plugin
  leash watch [path]       Add a folder to protect (default: cwd)
  leash watch --remove [path]
  leash undo               Restore files from the last burst in that folder
  leash status             Show daemon, agents, mission phase, pending approval
  leash demo [command]     Fake a dangerous hook (for recording)
  leash demo mission       Plan → tools → fail → gate (HUD demo)
  leash decide ID allow|always|kill
  leash steer TEXT         Inject operator note into the next tool
  leash interrupt [TEXT]   Kill the current/next tool
  leash retry              Ask the agent to retry the last failed tool
  leash skip               Dismiss the last failed tool
  leash always             List always-allow rules
  leash always --remove N  Revoke rule N from that list
  leash acp [--] <agent>   Permission socket in front of an ACP agent
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
	fmt.Println("ACP agents (no hook file): leash acp cursor|opencode|hermes|grok")
	printCensus(agents.Scan(agents.DefaultProbe()))
	fmt.Println("custom agent: see docs/INTEGRATION.md")
	return nil
}

func printCensus(found []agents.Found) {
	for _, a := range found {
		state := "missing"
		if a.Installed {
			state = "installed"
			switch {
			case a.Hooked && (a.Door == agents.DoorACP || a.Door == agents.DoorBoth):
				state = "hooks+acp"
			case a.Hooked:
				state = "hooks"
			case a.Door == agents.DoorACP || a.Door == agents.DoorBoth:
				state = "acp"
			}
		}
		line := fmt.Sprintf("  %-11s %s", a.Name, state)
		if a.Installed && a.ACP != "" && !a.Hooked {
			line += "  (leash acp -- " + a.ACP + ")"
		}
		fmt.Println(line)
	}
}

func cmdUninstall() error {
	if err := install.Uninstall(); err != nil {
		return err
	}
	fmt.Println("hooks removed")
	return nil
}

func cmdWatch() error {
	remove := false
	path := ""
	for _, a := range os.Args[2:] {
		if a == "-d" || a == "--remove" {
			remove = true
			continue
		}
		path = a
	}
	if path == "" {
		path, _ = os.Getwd()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	body := map[string]any{"path": abs}
	if remove {
		body["remove"] = true
	}
	return rpc("POST", "/v1/watch", body, nil)
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
			printCensus(agents.Scan(agents.DefaultProbe()))
			fmt.Println("start with: leash serve")
			return nil
		}
		return err
	}
	fmt.Printf("daemon: on  :%d  %s\n", st.Port, st.Status)
	if len(st.Agents) > 0 {
		printCensus(st.Agents)
	}
	if st.Mission.Phase != "" && st.Mission.Phase != "idle" {
		fmt.Printf("mission: %s", st.Mission.Phase)
		if st.Mission.Title != "" {
			fmt.Printf("  %s", st.Mission.Title)
		}
		if st.Mission.Agent != "" {
			fmt.Printf("  · %s", st.Mission.Agent)
		}
		fmt.Println()
		if st.Mission.Goal != "" {
			fmt.Printf("goal:    %s\n", st.Mission.Goal)
		}
	}
	if st.Job != nil {
		fmt.Printf("job:     %s  %s  %s\n", st.Job.Status, st.Job.Agent, st.Job.Task)
		if st.Job.FallbackFrom != "" {
			fmt.Printf("         fallback from %s\n", st.Job.FallbackFrom)
		}
		if st.Job.Error != "" {
			fmt.Printf("         %s\n", st.Job.Error)
		}
	}
	if live := st.Mission.Live; live != nil {
		label := live.Outcome
		if label == "" {
			label = live.Detail
		}
		fmt.Printf("live:    %s  %s\n", live.Status, label)
	}
	if f := st.Mission.Failed; f != nil {
		label := f.Outcome
		if label == "" {
			label = f.Tool
		}
		fmt.Printf("failed:  %s  %s\n", label, f.Error)
	}
	roots := st.WatchRoots
	if len(roots) == 0 && st.WatchRoot != "" {
		roots = []string{st.WatchRoot}
	}
	for _, r := range roots {
		fmt.Printf("watch:  %s\n", r)
	}
	if st.Waiting > 1 {
		fmt.Printf("waiting: %d\n", st.Waiting)
	}
	if st.Pending != nil {
		who := st.Pending.Title
		if st.Pending.Agent != "" {
			who = st.Pending.Agent + " · " + st.Pending.Title
		}
		fmt.Printf("pending %s  %s\n%s\n", st.Pending.ID, who, st.Pending.Detail)
		fmt.Println("decide: leash decide", st.Pending.ID, "allow|always|kill")
	}
	if st.Burst != nil {
		if st.Burst.Root != "" {
			fmt.Printf("result: %d files in %s\n", st.Burst.FileCount, st.Burst.Root)
		} else {
			fmt.Printf("result: %d files\n", st.Burst.FileCount)
		}
		for i, f := range st.Burst.Files {
			if i == 8 {
				fmt.Printf("        … %d more\n", len(st.Burst.Files)-8)
				break
			}
			fmt.Printf("        %s\n", f)
		}
	}
	if len(st.AlwaysAllow) > 0 {
		fmt.Println("always:")
		for i, r := range st.AlwaysAllow {
			line := fmt.Sprintf("  %d  %s  %s", i+1, r.Tool, r.Pattern)
			if r.Root != "" {
				line += "  " + r.Root
			}
			fmt.Println(line)
		}
	}
	return nil
}

func cmdAlways() error {
	args := os.Args[2:]
	remove := false
	idx := ""
	for _, a := range args {
		if a == "-d" || a == "--remove" {
			remove = true
			continue
		}
		idx = a
	}
	var st server.State
	if err := rpc("GET", "/v1/state", nil, &st); err != nil {
		return err
	}
	if !remove {
		if len(st.AlwaysAllow) == 0 {
			fmt.Println("no always-allow rules")
			return nil
		}
		for i, r := range st.AlwaysAllow {
			line := fmt.Sprintf("%d  %s  %s", i+1, r.Tool, r.Pattern)
			if r.Root != "" {
				line += "  " + r.Root
			}
			fmt.Println(line)
		}
		return nil
	}
	if idx == "" {
		return fmt.Errorf("usage: leash always --remove N")
	}
	n := 0
	for _, c := range idx {
		if c < '0' || c > '9' {
			return fmt.Errorf("usage: leash always --remove N")
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > len(st.AlwaysAllow) {
		return fmt.Errorf("no always rule %s", idx)
	}
	r := st.AlwaysAllow[n-1]
	var out server.State
	return rpc("POST", "/v1/always", map[string]any{
		"remove":  true,
		"tool":    r.Tool,
		"pattern": r.Pattern,
		"root":    r.Root,
	}, &out)
}

func cmdACP() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.File{}
	}
	launch, err := acp.ResolveLaunch(os.Args[2:], agents.DefaultProbe())
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	gate := daemonGate(cfg.Port, cfg.Token)
	notify := daemonNotify(cfg.Port, cfg.Token)
	fmt.Fprintf(os.Stderr, "leash acp: %s %s\n", launch.Command, strings.Join(launch.Args, " "))
	return acp.RunStdio(ctx, launch, gate, notify)
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
	if len(os.Args) > 2 && os.Args[2] == "mission" {
		return cmdDemoMission()
	}
	cmd := "rm -rf ./dist"
	if len(os.Args) > 2 {
		cmd = strings.Join(os.Args[2:], " ")
	}
	cwd, _ := os.Getwd()
	return postDemo(cwd, map[string]any{
		"session_id":      "demo",
		"cwd":             cwd,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"agent":           "Demo",
		"tool_input":      map[string]string{"command": cmd},
	}, 9*time.Minute)
}

func cmdDemoMission() error {
	cwd, _ := os.Getwd()
	fmt.Fprintln(os.Stderr, "demo mission: plan → act → fail → gate")
	steps := []map[string]any{
		{
			"protocol": "leash", "hook_event_name": "plan", "agent": "Demo", "cwd": cwd,
			"text": "Ship the login fix", "steps": []string{"read auth.ts", "run tests", "don't touch prod"},
		},
		{
			"protocol": "leash", "hook_event_name": "thought", "agent": "Demo", "cwd": cwd,
			"text": "checking middleware before the edit",
		},
		{
			"session_id": "demo", "cwd": cwd, "hook_event_name": "PreToolUse", "tool_name": "Write",
			"agent": "Demo", "tool_input": map[string]string{"file_path": cwd + "/auth.ts"},
		},
		{
			"protocol": "leash", "hook_event_name": "post_tool", "agent": "Demo", "cwd": cwd,
			"tool_name": "Write", "tool_input": map[string]string{"file_path": cwd + "/auth.ts"},
			"tool_output": "ok", "duration_ms": 40,
		},
		{
			"session_id": "demo", "cwd": cwd, "hook_event_name": "PreToolUse", "tool_name": "Bash",
			"agent": "Demo", "tool_input": map[string]string{"command": "git status"},
		},
		{
			"protocol": "leash", "hook_event_name": "post_tool", "agent": "Demo", "cwd": cwd,
			"tool_name": "Read", "tool_input": map[string]string{"file_path": cwd + "/auth.ts"},
			"tool_output": "export function auth() { return true }", "duration_ms": 12,
		},
		{
			"protocol": "leash", "hook_event_name": "post_tool", "agent": "Demo", "cwd": cwd,
			"tool_name": "Bash", "tool_input": map[string]string{"command": "npm test"},
			"error": "FAIL  auth.test.ts\n  expected 200, got 401\nexit status 1", "duration_ms": 1420,
		},
	}
	for _, body := range steps {
		if err := postDemo(cwd, body, 8*time.Second); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, "demo: rm -rf ./dist  (waiting on you)")
	return postDemo(cwd, map[string]any{
		"session_id": "demo", "cwd": cwd, "hook_event_name": "PreToolUse", "tool_name": "Bash",
		"agent": "Demo", "tool_input": map[string]string{"command": "rm -rf ./dist"},
	}, 9*time.Minute)
}

func postDemo(cwd string, payload map[string]any, timeout time.Duration) error {
	_ = cwd
	body, _ := json.Marshal(payload)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !server.DaemonRunning(cfg.Port) {
		return fmt.Errorf("daemon not running — start Leash.app or `leash serve`")
	}
	out, code, err := server.PostHook(cfg.Port, cfg.Token, body, timeout)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("http %d: %s", code, out)
	}
	if len(bytes.TrimSpace(out)) > 0 && !bytes.Equal(bytes.TrimSpace(out), []byte("{}")) {
		fmt.Println(string(bytes.TrimSpace(out)))
	}
	return nil
}

func cmdSteer() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: leash steer TEXT")
	}
	return rpc("POST", "/v1/steer", map[string]string{"text": strings.Join(os.Args[2:], " ")}, nil)
}

func cmdInterrupt() error {
	text := ""
	if len(os.Args) > 2 {
		text = strings.Join(os.Args[2:], " ")
	}
	return rpc("POST", "/v1/interrupt", map[string]string{"text": text}, nil)
}

func cmdRetry() error {
	return rpc("POST", "/v1/retry", map[string]any{}, nil)
}

func cmdSkip() error {
	return rpc("POST", "/v1/skip", map[string]any{}, nil)
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
