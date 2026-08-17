package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leashapp/leash/internal/atomicfile"
)

func ClaudeSettingsPath() string {
	return filepath.Join(home(), ".claude", "settings.json")
}

func CodexHooksPath() string {
	return filepath.Join(home(), ".codex", "hooks.json")
}

func CursorHooksPath() string {
	return filepath.Join(home(), ".cursor", "hooks.json")
}

func OpenCodePluginPath() string {
	return filepath.Join(home(), ".config", "opencode", "plugins", "leash.js")
}

func CodexConfigPath() string {
	return filepath.Join(home(), ".codex", "config.toml")
}

func home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func Install(bin string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 540
	}
	if err := mergeClaude(bin, timeoutSec); err != nil {
		return fmt.Errorf("claude settings: %w", err)
	}
	if err := mergeCodex(bin, timeoutSec); err != nil {
		return fmt.Errorf("codex hooks: %w", err)
	}
	if err := mergeCursor(bin, timeoutSec); err != nil {
		return fmt.Errorf("cursor hooks: %w", err)
	}
	if err := writeOpenCodePlugin(bin); err != nil {
		return fmt.Errorf("opencode plugin: %w", err)
	}
	return nil
}

func Uninstall() error {
	if err := stripClaude(); err != nil {
		return err
	}
	if err := stripCodex(); err != nil {
		return err
	}
	if err := stripCursor(); err != nil {
		return err
	}
	return removeOpenCodePlugin()
}

func mergeClaude(bin string, timeoutSec int) error {
	path := ClaudeSettingsPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	entry := map[string]any{
		"matcher": "*",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": hookCommand(bin),
				"timeout": timeoutSec,
			},
		},
	}
	hooks["PreToolUse"] = upsertGroup(asSlice(hooks["PreToolUse"]), entry)
	return writeJSON(path, root)
}

func stripClaude() error {
	path := ClaudeSettingsPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	hooks["PreToolUse"] = filterGroups(asSlice(hooks["PreToolUse"]))
	return writeJSON(path, root)
}

func mergeCodex(bin string, timeoutSec int) error {
	path := CodexHooksPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	cmd := hookCommand(bin)
	entry := map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{
				"type":          "command",
				"command":       cmd,
				"timeout":       timeoutSec,
				"statusMessage": "Leash",
			},
		},
	}
	patch := map[string]any{
		"matcher": "apply_patch",
		"hooks": []any{
			map[string]any{
				"type":          "command",
				"command":       cmd,
				"timeout":       timeoutSec,
				"statusMessage": "Leash",
			},
		},
	}
	hooks["PreToolUse"] = upsertGroup(asSlice(hooks["PreToolUse"]), entry)
	hooks["PreToolUse"] = upsertGroup(asSlice(hooks["PreToolUse"]), patch)
	hooks["PermissionRequest"] = upsertGroup(asSlice(hooks["PermissionRequest"]), entry)
	if err := writeJSON(path, root); err != nil {
		return err
	}
	return enableCodexHooks()
}

func stripCodex() error {
	path := CodexHooksPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	for _, k := range []string{"PreToolUse", "PermissionRequest"} {
		hooks[k] = filterGroups(asSlice(hooks[k]))
	}
	return writeJSON(path, root)
}

func enableCodexHooks() error {
	path := CodexConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	s := string(data)
	if strings.Contains(s, "codex_hooks") {
		return nil
	}
	block := "\n[features]\ncodex_hooks = true\n"
	if strings.Contains(s, "[features]") {
		s = strings.Replace(s, "[features]", "[features]\ncodex_hooks = true", 1)
		return atomicfile.WriteFile(path, []byte(s), 0o644)
	}
	return atomicfile.WriteFile(path, []byte(s+block), 0o644)
}

func hookCommand(bin string) string {
	return strconv.Quote(bin) + " hook"
}

func isLeashCommand(cmd string) bool {
	if strings.Contains(cmd, "leash hook") {
		return true
	}
	return strings.Contains(strings.ToLower(cmd), "leash") && strings.HasSuffix(strings.TrimSpace(cmd), " hook")
}

func upsertGroup(groups []any, entry map[string]any) []any {
	filtered := filterGroups(groups)
	return append(filtered, entry)
}

func filterGroups(groups []any) []any {
	var out []any
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			out = append(out, g)
			continue
		}
		if groupHasMarker(gm) {
			continue
		}
		out = append(out, g)
	}
	return out
}

func groupHasMarker(g map[string]any) bool {
	hooks := asSlice(g["hooks"])
	for _, h := range hooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if isLeashCommand(cmd) {
			return true
		}
	}
	return false
}

func asSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	default:
		return nil
	}
}

func mergeCursor(bin string, timeoutSec int) error {
	path := CursorHooksPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	if _, ok := root["version"]; !ok {
		root["version"] = 1
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		root["hooks"] = hooks
	}
	entry := map[string]any{
		"command": hookCommand(bin),
		"timeout": timeoutSec,
	}
	for _, ev := range []string{"preToolUse", "beforeShellExecution"} {
		hooks[ev] = upsertCommand(asSlice(hooks[ev]), entry)
	}
	return writeJSON(path, root)
}

func stripCursor() error {
	path := CursorHooksPath()
	root, err := readJSON(path)
	if err != nil {
		return err
	}
	hooks, _ := root["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}
	for _, ev := range []string{"preToolUse", "beforeShellExecution", "beforeMCPExecution", "beforeReadFile"} {
		hooks[ev] = filterCommands(asSlice(hooks[ev]))
	}
	return writeJSON(path, root)
}

func upsertCommand(list []any, entry map[string]any) []any {
	return append(filterCommands(list), entry)
}

func filterCommands(list []any) []any {
	var out []any
	for _, g := range list {
		gm, ok := g.(map[string]any)
		if !ok {
			out = append(out, g)
			continue
		}
		cmd, _ := gm["command"].(string)
		if isLeashCommand(cmd) {
			continue
		}
		out = append(out, g)
	}
	return out
}

func writeOpenCodePlugin(bin string) error {
	path := OpenCodePluginPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := strings.ReplaceAll(openCodePlugin, "__LEASH_BIN__", escapeJS(bin))
	return atomicfile.WriteFile(path, []byte(body), 0o644)
}

func removeOpenCodePlugin() error {
	path := OpenCodePluginPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !strings.Contains(string(data), "leash-plugin") {
		return nil
	}
	return os.Remove(path)
}

func escapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

const openCodePlugin = `// leash-plugin — generated by leash install. Do not edit.
import { spawnSync } from "node:child_process"

const LEASH = "__LEASH_BIN__"

function isDeny(out) {
  if (!out || typeof out !== "object") return false
  if (out.decision === "deny") return true
  if (out.permission === "deny") return true
  const spec = out.hookSpecificOutput
  if (spec && spec.permissionDecision === "deny") return true
  if (spec && spec.decision && spec.decision.behavior === "deny") return true
  return false
}

function reason(out) {
  if (!out) return "Blocked by Leash"
  return out.reason || out.agent_message || out.user_message ||
    (out.hookSpecificOutput && (out.hookSpecificOutput.permissionDecisionReason || (out.hookSpecificOutput.decision && out.hookSpecificOutput.decision.message))) ||
    "Blocked by Leash"
}

export const Leash = async ({ directory }) => {
  return {
    "tool.execute.before": async (input, output) => {
      const payload = {
        protocol: "leash",
        hook_event_name: "pre_tool",
        cwd: directory,
        tool_name: input.tool,
        tool_input: output.args || {},
      }
      const r = spawnSync(LEASH, ["hook"], {
        input: JSON.stringify(payload),
        encoding: "utf8",
        timeout: 540000,
      })
      if (r.error || r.status !== 0) return
      let out = {}
      try { out = JSON.parse(String(r.stdout || "{}").trim() || "{}") } catch { return }
      if (isDeny(out)) {
        throw new Error(reason(out))
      }
    },
  }
}
`

func readJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

func writeJSON(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, append(data, '\n'), 0o644)
}
