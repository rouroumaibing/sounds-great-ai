package claude

import (
	"context"
	"strings"
	"testing"

	"sounds-great-ai/internal/adapter/unified"

	"github.com/cloudwego/eino/schema"
)

func TestAdapterCapabilities(t *testing.T) {
	a := New(nil)
	caps := a.Capabilities()
	if !caps.SupportsMCP {
		t.Error("Claude Code should support MCP")
	}
	if caps.OutputFormat != "stream-json" {
		t.Errorf("output format = %s, want stream-json", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "claude-not-installed"
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestAdapterBuildArgs(t *testing.T) {
	a := New(nil)
	args, _ := a.buildArgs("claude-opus-4-6", "/tmp/work", nil, "")
	found := false
	for _, arg := range args {
		if arg == "stream-json" {
			found = true
		}
	}
	if !found {
		t.Error("expected stream-json in args")
	}
}

// --- New edge case and error case tests ---

func argsContains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

func TestAdapterNewDefaults(t *testing.T) {
	a := New(nil)
	if a.BinaryPath != "claude" {
		t.Errorf("BinaryPath = %q, want %q", a.BinaryPath, "claude")
	}
}

func TestAdapterBuildArgsTableDriven(t *testing.T) {
	a := New(nil)
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		model          string
		workDir        string
		mcp            *unified.MCPConfig
		systemPrompt   string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "all empty",
			wantContains:   []string{"--output-format", "stream-json"},
			wantNotContain: []string{"--model", "--cwd", "--append-system-prompt", "--mcp-config"},
		},
		{
			name:           "with model only",
			model:          "claude-opus-4-6",
			wantContains:   []string{"--output-format", "stream-json", "--model", "claude-opus-4-6"},
			wantNotContain: []string{"--cwd", "--append-system-prompt", "--mcp-config"},
		},
		{
			name:           "with workDir only",
			workDir:        "/tmp/work",
			wantContains:   []string{"--output-format", "stream-json", "--cwd", "/tmp/work"},
			wantNotContain: []string{"--model", "--append-system-prompt", "--mcp-config"},
		},
		{
			name:           "with system prompt only",
			systemPrompt:   "You are a helpful assistant",
			wantContains:   []string{"--output-format", "stream-json", "--append-system-prompt", "You are a helpful assistant"},
			wantNotContain: []string{"--model", "--cwd", "--mcp-config"},
		},
		{
			name:           "with all fields set",
			model:          "claude-opus-4-6",
			workDir:        "/tmp/work",
			systemPrompt:   "You are a helpful assistant",
			wantContains:   []string{"--output-format", "stream-json", "--model", "claude-opus-4-6", "--cwd", "/tmp/work", "--append-system-prompt", "You are a helpful assistant"},
			wantNotContain: []string{"--mcp-config"},
		},
		{
			name:    "with mcp config and servers",
			workDir: tmpDir,
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{
					{Name: "test-server", Command: "echo"},
				},
			},
			wantContains:   []string{"--output-format", "stream-json", "--cwd", "--mcp-config"},
			wantNotContain: []string{"--model", "--append-system-prompt"},
		},
		{
			name:           "with nil mcp config",
			model:          "claude-opus-4-6",
			workDir:        "/tmp/work",
			wantContains:   []string{"--output-format", "stream-json", "--model", "claude-opus-4-6", "--cwd", "/tmp/work"},
			wantNotContain: []string{"--mcp-config", "--append-system-prompt"},
		},
		{
			name:    "with mcp config but no servers",
			workDir: "/tmp/work",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{},
			},
			wantContains:   []string{"--output-format", "stream-json", "--cwd", "/tmp/work"},
			wantNotContain: []string{"--mcp-config", "--model", "--append-system-prompt"},
		},
		{
			name:    "with mcp config but empty workDir",
			workDir: "",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{
					{Name: "test-server", Command: "echo"},
				},
			},
			wantContains:   []string{"--output-format", "stream-json"},
			wantNotContain: []string{"--mcp-config", "--model", "--cwd", "--append-system-prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := a.buildArgs(tt.model, tt.workDir, tt.mcp, tt.systemPrompt)
			for _, want := range tt.wantContains {
				if !argsContains(args, want) {
					t.Errorf("args = %v, want to contain %q", args, want)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if argsContains(args, notWant) {
					t.Errorf("args = %v, should not contain %q", args, notWant)
				}
			}
		})
	}
}

func TestAdapterBuildStdin(t *testing.T) {
	a := New(nil)

	tests := []struct {
		name string
		req  unified.ExecuteRequest
		want string
	}{
		{
			name: "empty request",
			req:  unified.ExecuteRequest{},
			want: "",
		},
		{
			name: "system prompt only",
			req: unified.ExecuteRequest{
				SystemPrompt: "You are a dog",
			},
			want: "You are a dog\n\n",
		},
		{
			name: "messages only",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: "Hello"},
					{Content: "World"},
				},
			},
			want: "Hello\nWorld\n",
		},
		{
			name: "system prompt and messages",
			req: unified.ExecuteRequest{
				SystemPrompt: "You are a dog",
				Messages: []*schema.Message{
					{Content: "Fetch the ball"},
				},
			},
			want: "You are a dog\n\nFetch the ball\n",
		},
		{
			name: "empty message content",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: ""},
				},
			},
			want: "\n",
		},
		{
			name: "multiple empty messages",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: ""},
					{Content: ""},
				},
			},
			want: "\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.buildStdin(tt.req)
			if got != tt.want {
				t.Errorf("buildStdin() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAdapterExecuteNilProcessManager(t *testing.T) {
	a := New(nil)
	_, err := a.Execute(context.Background(), unified.ExecuteRequest{})
	if err == nil {
		t.Fatal("expected error when process manager is nil")
	}
	if !strings.Contains(err.Error(), "process manager not configured") {
		t.Errorf("error = %v, want process manager not configured", err)
	}
}

func TestAdapterCapabilitiesConsistency(t *testing.T) {
	a := New(nil)
	caps1 := a.Capabilities()
	caps2 := a.Capabilities()
	if caps1 != caps2 {
		t.Error("capabilities should be consistent across calls")
	}
	if !caps1.SupportsTools {
		t.Error("Claude Code should support tools")
	}
	if !caps1.SupportsFileOps {
		t.Error("Claude Code should support file ops")
	}
}

func TestAdapterHealthEmptyBinaryPath(t *testing.T) {
	a := New(nil)
	a.BinaryPath = ""
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for empty binary path")
	}
}

func TestAdapterHealthCancelledContext(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "claude-not-installed"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Health(ctx); err == nil {
		t.Error("expected error for missing binary even with cancelled context")
	}
}

func TestParseClaudeEvent(t *testing.T) {
	tests := []struct {
		name        string
		obj         map[string]any
		wantType    string
		wantContent string
	}{
		{
			name:        "assistant_response",
			obj:         map[string]any{"type": "assistant_response", "content": "Hello world"},
			wantType:    "text",
			wantContent: "Hello world",
		},
		{
			name:        "tool_use",
			obj:         map[string]any{"type": "tool_use", "name": "bash"},
			wantType:    "tool_call",
			wantContent: "",
		},
		{
			name:        "tool_result",
			obj:         map[string]any{"type": "tool_result", "result": "done"},
			wantType:    "tool_result",
			wantContent: "",
		},
		{
			name:        "result event",
			obj:         map[string]any{"type": "result", "data": "final"},
			wantType:    "done",
			wantContent: "",
		},
		{
			name:        "unknown type",
			obj:         map[string]any{"type": "unknown", "data": "test"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "missing type field",
			obj:         map[string]any{"content": "test"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "assistant_response missing content",
			obj:         map[string]any{"type": "assistant_response"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "assistant_response non-string content",
			obj:         map[string]any{"type": "assistant_response", "content": 123},
			wantType:    "text",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := parseClaudeEvent(tt.obj)
			if evt.Type != tt.wantType {
				t.Errorf("type = %q, want %q", evt.Type, tt.wantType)
			}
			if evt.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", evt.Content, tt.wantContent)
			}
		})
	}
}
