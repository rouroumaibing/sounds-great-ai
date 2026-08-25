package kimi

import (
	"strings"
	"testing"

	"sounds-great-ai/internal/adapter/unified"
)

// sampleKimiStream mimics `kimi -p ... --output-format stream-json` NDJSON output:
// an assistant message with text, a thinking block, a tool call, a usage stat,
// and a session id.
const sampleKimiStream = `{"role":"assistant","content":"Hello from Kimi","thinking":"let me think...","session_id":"sess-123"}
{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":"{\"path\":\"/tmp/x\"}"}}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}
{"role":"meta","type":"session.resume_hint","session_id":"sess-456"}
`

func TestStreamEvents_ParsesKimiNDJSON(t *testing.T) {
	ch := parseSample(sampleKimiStream)

	var types []string
	var toolName, toolArgs, thinking, text string
	var sessionID string
	for ev := range ch {
		types = append(types, ev.Type)
		switch ev.Type {
		case "text":
			text = ev.Content
		case "thinking":
			thinking = ev.Content
		case "tool_call":
			toolName, _ = ev.Meta["tool"].(string)
			toolArgs = ev.Content
		case "done":
			if sid, ok := ev.Meta["session_id"].(string); ok {
				sessionID = sid
			}
		}
	}

	if text != "Hello from Kimi" {
		t.Errorf("text = %q, want %q", text, "Hello from Kimi")
	}
	if thinking != "let me think..." {
		t.Errorf("thinking = %q, want %q", thinking, "let me think...")
	}
	if toolName != "read_file" {
		t.Errorf("tool name = %q, want read_file", toolName)
	}
	if !strings.Contains(toolArgs, "/tmp/x") {
		t.Errorf("tool args = %q, want it to contain /tmp/x", toolArgs)
	}
	if sessionID == "" {
		t.Errorf("expected session_id to be captured into done meta, got empty")
	}
	// text, thinking, tool_call, done => 4 events.
	if len(types) != 4 {
		t.Errorf("event count = %d, want 4 (got %v)", len(types), types)
	}
}

func TestExtractTextContent_Blocks(t *testing.T) {
	in := []any{
		map[string]any{"type": "text", "text": "a"},
		map[string]any{"content": "b"},
		"c",
	}
	if got := extractTextContent(in); got != "a\nb\nc" {
		t.Errorf("extractTextContent = %q, want %q", got, "a\nb\nc")
	}
}

func TestParseUsage_PicksKnownKeys(t *testing.T) {
	u := parseUsage(map[string]any{"input_tokens": 3.0, "output_tokens": 7.0, "total_tokens": 10.0})
	if u == nil || u["input_tokens"] != 3.0 || u["output_tokens"] != 7.0 {
		t.Errorf("parseUsage = %v", u)
	}
	if parseUsage(map[string]any{"unrelated": 1.0}) != nil {
		t.Errorf("expected nil for unknown keys")
	}
}

// parseSample drives the private streamEvents parser via a fake SpawnHandle.
func parseSample(s string) <-chan unified.StreamEvent {
	// streamEvents now reads from SpawnHandle.Stdout; feed it a fake handle
	// carrying the sample payload as its stdout reader.
	a := &Adapter{}
	return a.streamEvents(&unified.SpawnHandle{Stdout: strings.NewReader(s)})
}

// --- F274: Kimi native L0 channel (--agent-file) ---

func TestKimi_NativeL0Channel(t *testing.T) {
	// Override the file writer so the test is hermetic (no real temp files).
	old := l0FileWriter
	defer func() { l0FileWriter = old }()
	l0FileWriter = func(content string) (string, error) { return "/tmp/fake-kimi-l0.md", nil }

	a := &Adapter{}
	withL0 := a.buildArgs("m1", "", "prompt", "native L0 system prompt")
	if !containsArg(withL0, "--agent-file") {
		t.Fatalf("native L0 must append --agent-file, got %v", withL0)
	}
	// Without L0, no --agent-file flag is emitted.
	withoutL0 := a.buildArgs("m1", "", "prompt", "")
	for _, arg := range withoutL0 {
		if arg == "--agent-file" {
			t.Fatalf("L0-absent build must not emit --agent-file, got %v", withoutL0)
		}
	}
}

func TestKimi_SupportsNativeL0(t *testing.T) {
	a := &Adapter{}
	if !a.Capabilities().SupportsNativeL0 {
		t.Fatal("Kimi adapter must advertise SupportsNativeL0 (F274)")
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
