//go:build pty

package unified

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupHookInfrastructure_CreatesAndCleansUp(t *testing.T) {
	cwd := t.TempDir()
	sidecar := filepath.Join(t.TempDir(), "sidecar.jsonl")

	res, err := SetupHookInfrastructure(cwd, sidecar)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// settings.json should exist and reference our capture script for both hooks.
	b, err := os.ReadFile(res.SettingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatalf("no hooks key in settings: %s", b)
	}
	for _, kind := range []string{"Stop", "PostToolUse"} {
		entries, ok := hooks[kind].([]any)
		if !ok || len(entries) == 0 {
			t.Fatalf("hook %s missing", kind)
		}
		entry := entries[0].(map[string]any)
		h := entry["hooks"].([]any)[0].(map[string]any)
		if got := h["command"].(string); got != res.ScriptPath {
			t.Fatalf("hook %s command = %q, want %q", kind, got, res.ScriptPath)
		}
	}
	if _, err := os.Stat(res.ScriptPath); err != nil {
		t.Fatalf("capture script missing: %v", err)
	}

	// Cleanup should remove the settings file we created and the script.
	res.Cleanup()
	if _, err := os.Stat(res.SettingsPath); !os.IsNotExist(err) {
		t.Fatalf("settings not removed after cleanup")
	}
	if _, err := os.Stat(res.ScriptPath); !os.IsNotExist(err) {
		t.Fatalf("script not removed after cleanup")
	}
}

func TestSetupHookInfrastructure_PreservesExistingSettings(t *testing.T) {
	cwd := t.TempDir()
	sidecar := filepath.Join(t.TempDir(), "sidecar.jsonl")
	orig := `{"model":"opus","hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"/usr/bin/echo"}]}]}}`
	if err := os.MkdirAll(filepath.Join(cwd, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".claude", "settings.json"), []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := SetupHookInfrastructure(cwd, sidecar)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Our Stop/PostToolUse hooks were added without dropping PreToolUse.
	b, _ := os.ReadFile(res.SettingsPath)
	var doc map[string]any
	_ = json.Unmarshal(b, &doc)
	hooks := doc["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Fatalf("PreToolUse was dropped: %s", b)
	}
	if doc["model"] != "opus" {
		t.Fatalf("model key dropped: %s", b)
	}

	res.Cleanup()
	// Original settings restored verbatim.
	restored, err := os.ReadFile(res.SettingsPath)
	if err != nil {
		t.Fatalf("original settings not restored: %v", err)
	}
	if strings.TrimSpace(string(restored)) != strings.TrimSpace(orig) {
		t.Fatalf("restored settings mismatch:\n got %s\nwant %s", restored, orig)
	}
}

func TestHookSidechannelParseAndMap(t *testing.T) {
	sidecar := filepath.Join(t.TempDir(), "sidecar.jsonl")
	events := []HookEvent{
		{
			EventType:  "PostToolUse",
			SessionID:  "sess-abc",
			ToolName:   "Read",
			ToolUseID:  "tu-1",
			ToolInput:  map[string]any{"file_path": "/x"},
		},
		{
			EventType:            "Stop",
			SessionID:            "sess-abc",
			LastAssistantMessage: "All done.",
			Entrypoint:           "cli",
		},
	}
	var sb strings.Builder
	for _, e := range events {
		line, _ := json.Marshal(e)
		sb.Write(line)
		sb.WriteByte('\n')
	}
	if err := os.WriteFile(sidecar, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseHookSidecar(sidecar)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("parsed %d events, want 2", len(parsed))
	}
	if !IsHookTerminal(parsed[1]) {
		t.Fatalf("Stop event should be terminal")
	}
	if got := ExtractHookSessionID(parsed); got != "sess-abc" {
		t.Fatalf("session id = %q, want sess-abc", got)
	}

	mapped := MapHookEventsToStream(parsed)
	// Expect: tool_call (PostToolUse) + text (Stop message) + done (Stop).
	if len(mapped) != 3 {
		t.Fatalf("mapped %d events, want 3: %+v", len(mapped), mapped)
	}
	if mapped[0].Type != "tool_call" || mapped[0].Content != "Read" {
		t.Fatalf("first mapped = %+v, want tool_call/Read", mapped[0])
	}
	if mapped[1].Type != "text" || mapped[1].Content != "All done." {
		t.Fatalf("second mapped = %+v, want text/All done.", mapped[1])
	}
	if mapped[2].Type != "done" {
		t.Fatalf("third mapped = %+v, want done", mapped[2])
	}
	if mapped[2].Meta["entrypoint"] != "cli" {
		t.Fatalf("done entrypoint = %v, want cli", mapped[2].Meta["entrypoint"])
	}
}
