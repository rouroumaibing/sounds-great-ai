//go:build pty

package unified

// pty_hook.go implements the Claude Code Hook side-channel, the
// structural output path used by its PtyDriver (F230) for claude >= 2.1.172,
// where the interactive TUI no longer writes transcript files.
//
// Mechanism (faithful to the upstream pty/hook-setup.ts + HookSidechannelConsumer.ts design):
//  1. Before launching claude, we drop a scoped `.claude/settings.json` into the
//     cwd registering `Stop` and `PostToolUse` hooks that invoke a POSIX capture
//     script. The original settings (if any) are backed up and restored on cleanup.
//  2. The capture script reads each hook event JSON from stdin and appends it to
//     $SG_HOOK_SIDECAR (a sidecar jsonl file).
//  3. A tailer reads that jsonl and converts the events into SG StreamEvents:
//     `Stop` is the terminal signal (carries the final assistant message),
//     `PostToolUse` surfaces tool invisibility, and every event carries a
//     session_id used for session discovery.
//
// This runs ONLY under `-tags pty` and is claude-specific; it supplements (does
// not replace) the direct NDJSON read from the PTY master.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvHookSidecar is the env var pointing the capture script at the sidecar
// jsonl file. It mirrors the CAT_CAFE_HOOK_SIDECAR contract.
const EnvHookSidecar = "SG_HOOK_SIDECAR"

// HookEvent is a single Claude Code hook event as captured from stdin.
type HookEvent struct {
	EventType    string `json:"hook_event_name"` // "Stop" | "PostToolUse" | ...
	SessionID    string `json:"session_id"`
	// Stop payload
	LastAssistantMessage string `json:"last_assistant_message"`
	// PostToolUse payload
	ToolName  string `json:"tool_name"`
	ToolInput any    `json:"tool_input"`
	ToolUseID string `json:"tool_use_id"`
	// Entrypoint (billing identity) injected by the capture script.
	Entrypoint string `json:"_cc_entrypoint"`
}

// HookInfrastructureResult holds the temp artifacts created for the side
// channel and a Cleanup that restores the original settings and removes the
// capture script.
type HookInfrastructureResult struct {
	SettingsPath string
	ScriptPath   string
	SidecarPath  string
	cleanup      func()
}

// Cleanup restores the original .claude/settings.json (or removes the file if
// we created it) and deletes the capture script.
func (r *HookInfrastructureResult) Cleanup() {
	if r.cleanup != nil {
		r.cleanup()
	}
}

// captureScript is a POSIX sh script that appends each hook event (tagged with
// the Claude Code entrypoint) to the sidecar jsonl. It is intentionally tiny
// and dependency-free so it runs in any shell.
const captureScript = `#!/bin/sh
# sg-cli hook capture: append hook event JSON (stdin) to the sidecar file.
SIDECAR="$SG_HOOK_SIDECAR"
[ -z "$SIDECAR" ] && exit 0
LINE=$(cat)
[ -z "$LINE" ] && exit 0
if [ -n "$CLAUDE_CODE_ENTRYPOINT" ]; then
  printf '%s' "$LINE" | sed "s/}$/,\"_cc_entrypoint\":\"$CLAUDE_CODE_ENTRYPOINT\"}/" >> "$SIDECAR"
else
  printf '%s\n' "$LINE" >> "$SIDECAR"
fi
`

// SetupHookInfrastructure writes a scoped .claude/settings.json (backing up any
// existing one) and the capture script into cwd, returning a result whose
// Cleanup restores the original state. The sidecar jsonl path is propagated to
// the capture script via the SG_HOOK_SIDECAR env var (set by the caller when
// spawning claude), NOT baked into the command.
func SetupHookInfrastructure(cwd, sidecarPath string) (*HookInfrastructureResult, error) {
	claudeDir := filepath.Join(cwd, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Backup any existing settings so we can restore on cleanup.
	var orig []byte
	createdSettings := false
	if b, err := os.ReadFile(settingsPath); err == nil {
		orig = b
	} else if os.IsNotExist(err) {
		createdSettings = true
	} else {
		return nil, fmt.Errorf("read existing settings: %w", err)
	}

	// Capture script in a temp location (outside .claude to avoid polluting
	// the user's repo config dir).
	scriptFile, err := os.CreateTemp("", "sg-hook-capture-*.sh")
	if err != nil {
		return nil, fmt.Errorf("create capture script: %w", err)
	}
	scriptPath := scriptFile.Name()
	if _, err := scriptFile.WriteString(captureScript); err != nil {
		_ = scriptFile.Close()
		_ = os.Remove(scriptPath)
		return nil, fmt.Errorf("write capture script: %w", err)
	}
	_ = scriptFile.Close()
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		_ = os.Remove(scriptPath)
		return nil, fmt.Errorf("chmod capture script: %w", err)
	}

	merged, err := mergeHookSettings(orig, scriptPath)
	if err != nil {
		_ = os.Remove(scriptPath)
		return nil, err
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		_ = os.Remove(scriptPath)
		return nil, fmt.Errorf("mkdir .claude: %w", err)
	}
	if err := os.WriteFile(settingsPath, merged, 0o644); err != nil {
		_ = os.Remove(scriptPath)
		_ = os.Remove(settingsPath)
		return nil, fmt.Errorf("write settings: %w", err)
	}

	res := &HookInfrastructureResult{
		SettingsPath: settingsPath,
		ScriptPath:   scriptPath,
		SidecarPath:  sidecarPath,
	}
	res.cleanup = func() {
		_ = os.Remove(scriptPath)
		if createdSettings {
			_ = os.Remove(settingsPath)
			// Best-effort remove the now-empty .claude dir.
			_ = os.Remove(claudeDir)
		} else if orig != nil {
			_ = os.WriteFile(settingsPath, orig, 0o644)
		}
	}
	return res, nil
}

// mergeHookSettings injects our Stop/PostToolUse hooks into existing settings
// JSON (or creates a minimal settings doc), without clobbering other keys. The
// hook command invokes the capture script (scriptPath); the sidecar jsonl is
// supplied to that script via the SG_HOOK_SIDECAR env var.
func mergeHookSettings(existing []byte, scriptPath string) ([]byte, error) {
	hookEntry := map[string]any{
		"hooks": []map[string]any{
			{"type": "command", "command": scriptPath},
		},
	}
	var doc map[string]any
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &doc); err != nil {
			return nil, fmt.Errorf("parse existing settings: %w", err)
		}
	} else {
		doc = map[string]any{}
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		doc["hooks"] = hooks
	}
	hooks["Stop"] = []map[string]any{hookEntry}
	hooks["PostToolUse"] = []map[string]any{hookEntry}
	return json.MarshalIndent(doc, "", "  ")
}

// ParseHookSidecar reads all hook events captured so far from the sidecar jsonl.
func ParseHookSidecar(path string) ([]HookEvent, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []HookEvent
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e HookEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		events = append(events, e)
	}
	return events, nil
}

// IsHookTerminal reports whether the event is a terminal (Stop) signal.
func IsHookTerminal(e HookEvent) bool {
	return e.EventType == "Stop"
}

// ExtractHookSessionID returns the first non-empty session_id across events
// (the backup session-discovery mechanism when transcript polling fails).
func ExtractHookSessionID(events []HookEvent) string {
	for _, e := range events {
		if e.SessionID != "" {
			return e.SessionID
		}
	}
	return ""
}

// MapHookEventsToStream converts hook events into SG StreamEvents. Stop becomes
// a `done` event carrying the final assistant message (plus a `text` event when
// the message is non-empty); PostToolUse becomes a `tool_call` event exposing
// tool visibility. Terminal detection is left to the tailer, which emits `done`
// on Stop.
func MapHookEventsToStream(events []HookEvent) []StreamEvent {
	var out []StreamEvent
	for _, e := range events {
		switch e.EventType {
		case "Stop":
			if msg := strings.TrimSpace(e.LastAssistantMessage); msg != "" {
				out = append(out, StreamEvent{Type: "text", Content: msg})
			}
			out = append(out, StreamEvent{Type: "done", Meta: map[string]any{
				"session_id": e.SessionID,
				"entrypoint": e.Entrypoint,
			}})
		case "PostToolUse":
			out = append(out, StreamEvent{
				Type:    "tool_call",
				Content: e.ToolName,
				Meta: map[string]any{
					"tool_name":   e.ToolName,
					"tool_use_id": e.ToolUseID,
					"tool_input":  e.ToolInput,
					"session_id":  e.SessionID,
				},
			})
		}
	}
	return out
}
