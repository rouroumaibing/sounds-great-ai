//go:build pty

package unified

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTranscriptSlug(t *testing.T) {
	cases := map[string]string{
		"/home/u/proj":        "-home-u-proj",
		"/Users/foo/bar/baz":  "-Users-foo-bar-baz",
		"/":                   "-",
		"/a":                  "-a",
	}
	for in, want := range cases {
		if got := transcriptSlug(in); got != want {
			t.Errorf("transcriptSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"plain":      "'plain'",
		"a b":        "'a b'",
		"it's":       "'it'\\''s'",
		"$(rm -rf)":  "'$(rm -rf)'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTranscriptLine(t *testing.T) {
	line := `{"type":"assistant","sessionId":"sess-123","message":{"role":"assistant","content":[{"type":"text","text":"hello world"},{"type":"tool_use","id":"t1","name":"Read","input":{"path":"/x"}}]}}`
	sid, evs := parseTranscriptLine(line)
	if sid != "sess-123" {
		t.Errorf("sessionId = %q, want sess-123", sid)
	}
	if len(evs) != 2 {
		t.Fatalf("expected 2 events, got %d", len(evs))
	}
	if evs[0].OutType != "text" || evs[0].Content != "hello world" {
		t.Errorf("first event = %+v", evs[0])
	}
	if evs[1].OutType != "tool_call" {
		t.Errorf("second event = %+v", evs[1])
	}

	toolLine := `{"type":"tool","sessionId":"sess-123","message":{"role":"tool","content":[{"type":"text","text":"file contents"}]}}`
	sid2, evs2 := parseTranscriptLine(toolLine)
	if sid2 != "sess-123" || len(evs2) != 1 || evs2[0].OutType != "tool_result" {
		t.Errorf("tool line parse: sid=%q evs=%+v", sid2, evs2)
	}

	// Unrelated line shapes yield no events.
	if _, evs3 := parseTranscriptLine(`{"type":"user","message":{"role":"user"}}`); len(evs3) != 0 {
		t.Errorf("user line should yield no events, got %d", len(evs3))
	}
	if _, evs4 := parseTranscriptLine(`not json`); len(evs4) != 0 {
		t.Errorf("malformed line should yield no events, got %d", len(evs4))
	}
}

func TestBuildClaudeTmuxCommand(t *testing.T) {
	spec := &SpawnSpec{
		Command:    "claude",
		Args:       []string{"--output-format", "stream-json", "--model", "opus", "--cwd", "/w"},
		StdinInput: "do the thing",
	}
	cmd, err := buildClaudeTmuxCommand(spec, "/tmp/sidecar.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "SG_HOOK_SIDECAR='/tmp/sidecar.jsonl'") {
		t.Errorf("missing inline sidecar env: %q", cmd)
	}
	if strings.Contains(cmd, "--output-format") {
		t.Errorf("should drop --output-format: %q", cmd)
	}
	if !strings.Contains(cmd, "--model") || !strings.Contains(cmd, "opus") {
		t.Errorf("should keep --model opus: %q", cmd)
	}
	if !strings.Contains(cmd, "'do the thing'") {
		t.Errorf("prompt should be positional single-quoted: %q", cmd)
	}
	// command itself single-quoted
	if !strings.Contains(cmd, "'claude'") {
		t.Errorf("command should be quoted: %q", cmd)
	}
}

func TestNewestTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.jsonl")
	_ = os.WriteFile(old, []byte("x"), 0o644)
	time.Sleep(10 * time.Millisecond)
	newer := filepath.Join(dir, "newer.jsonl")
	_ = os.WriteFile(newer, []byte("yy"), 0o644)
	// empty file should be ignored
	empty := filepath.Join(dir, "empty.jsonl")
	_ = os.WriteFile(empty, nil, 0o644)

	got, ok := newestTranscriptFile(dir)
	if !ok || got != newer {
		t.Errorf("newestTranscriptFile = (%q, %v), want (%q, true)", got, ok, newer)
	}

	if _, ok := newestTranscriptFile(t.TempDir()); ok {
		t.Errorf("empty dir should return false")
	}
}

// TestTmuxCoordinatorWithTranscript drives the merge coordinator with synthetic
// transcript + hook files (no real tmux) and verifies the merged NDJSON stream:
// transcript text + tool_use are emitted, the hook Stop is the terminal signal,
// the final answer is NOT double-emitted (transcript already delivered it), and
// a terminal `result` closes the pipe.
func TestTmuxCoordinatorWithTranscript(t *testing.T) {
	transcript := t.TempDir() + "/session.jsonl"
	_ = os.WriteFile(transcript, []byte(
		`{"type":"assistant","sessionId":"S1","message":{"role":"assistant","content":[{"type":"text","text":"working on it"},{"type":"tool_use","id":"t1","name":"Bash","input":{"cmd":"ls"}}]}}`+"\n",
	), 0o644)

	sidecar := t.TempDir() + "/sidecar.jsonl"
	stop := HookEvent{EventType: "Stop", SessionID: "S1", LastAssistantMessage: "working on it"}
	stopBytes, _ := json.Marshal(stop)
	_ = os.WriteFile(sidecar, append(stopBytes, '\n'), 0o644)

	c, handle := newTmuxStreamCoordinator(context.Background(), "", nil, 0)

	go tailFile(c.ctx, transcript, func(line string) {
		sid, evs := parseTranscriptLine(line)
		if sid != "" {
			c.setSessionID(sid)
		}
		for _, e := range evs {
			c.writeTranscriptEvent(e)
		}
	}, nil)
	go tailFile(c.ctx, sidecar, func(line string) {
		var e HookEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return
		}
		if e.SessionID != "" {
			c.setSessionID(e.SessionID)
		}
		if e.EventType == "Stop" {
			c.onHookStop(e)
		}
	}, nil)

	// Read the merged stream until the pipe closes.
	type evt struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	got := collectEventsFromReader(t, handle.Stdout)

	if len(got) < 3 {
		t.Fatalf("expected at least 3 events, got %d: %+v", len(got), got)
	}
	// 1) assistant_response text
	if got[0].Type != "assistant_response" || got[0].Content != "working on it" {
		t.Errorf("event0 = %+v", got[0])
	}
	// 2) tool_use
	if got[1].Type != "tool_use" {
		t.Errorf("event1 = %+v", got[1])
	}
	// 3) terminal result
	last := got[len(got)-1]
	if last.Type != "result" {
		t.Errorf("last event should be result, got %+v", last)
	}
	// No duplicate final text (transcript already delivered it).
	for _, e := range got {
		if e.Type == "assistant_response" && e.Content == "working on it" {
			// exactly one occurrence expected (from transcript)
		}
	}
	count := 0
	for _, e := range got {
		if e.Type == "assistant_response" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 assistant_response (no hook dup), got %d", count)
	}
}

// TestTmuxCoordinatorNoTranscript drives the coordinator when the transcript is
// absent: the hook Stop must still deliver the final answer text + terminal.
func TestTmuxCoordinatorNoTranscript(t *testing.T) {
	sidecar := t.TempDir() + "/sidecar.jsonl"
	stop := HookEvent{EventType: "Stop", SessionID: "S2", LastAssistantMessage: "final answer here"}
	stopBytes, _ := json.Marshal(stop)
	_ = os.WriteFile(sidecar, append(stopBytes, '\n'), 0o644)

	c, handle := newTmuxStreamCoordinator(context.Background(), "", nil, 0)
	go tailFile(c.ctx, sidecar, func(line string) {
		var e HookEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return
		}
		if e.SessionID != "" {
			c.setSessionID(e.SessionID)
		}
		if e.EventType == "Stop" {
			c.onHookStop(e)
		}
	}, nil)

	got := collectEventsFromReader(t, handle.Stdout)
	if len(got) < 2 {
		t.Fatalf("expected >=2 events, got %d: %+v", len(got), got)
	}
	// final answer recovered from the hook
	if got[0].Type != "assistant_response" || got[0].Content != "final answer here" {
		t.Errorf("event0 = %+v", got[0])
	}
	if got[len(got)-1].Type != "result" {
		t.Errorf("last event should be result, got %+v", got[len(got)-1])
	}
}

// collectEventsFromReader reads NDJSON lines from r until EOF (pipe close).
type tmuxTestEvent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func collectEventsFromReader(t *testing.T, r interface {
	Read([]byte) (int, error)
}) []tmuxTestEvent {
	t.Helper()
	var out []tmuxTestEvent
	buf := make([]byte, 4096)
	var leftover string
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			return out
		default:
		}
		n, err := r.Read(buf)
		if n > 0 {
			leftover += string(buf[:n])
			for {
				idx := strings.IndexByte(leftover, '\n')
				if idx < 0 {
					break
				}
				line := strings.TrimSpace(leftover[:idx])
				leftover = leftover[idx+1:]
				if line == "" {
					continue
				}
				var e tmuxTestEvent
				if jErr := json.Unmarshal([]byte(line), &e); jErr == nil {
					out = append(out, e)
				}
			}
		}
		if err != nil {
			return out
		}
	}
}
