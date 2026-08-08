package codex

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/adapter/unified"
)

func TestAdapterCapabilities(t *testing.T) {
	a := New(nil)
	caps := a.Capabilities()
	if !caps.SupportsMCP {
		t.Error("Codex CLI should support MCP")
	}
	if caps.OutputFormat != "json" {
		t.Errorf("output format = %s, want json", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "codex-not-installed"
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for missing binary")
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
	if a.BinaryPath != "codex" {
		t.Errorf("BinaryPath = %q, want %q", a.BinaryPath, "codex")
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
			wantContains:   []string{"exec", "--json"},
			wantNotContain: []string{"--model", "--mcp-config", "-c"},
		},
		{
			name:           "with model only",
			model:          "gpt-4o",
			wantContains:   []string{"exec", "--json", "--model", "gpt-4o"},
			wantNotContain: []string{"--mcp-config", "-c"},
		},
		{
			name:           "with system prompt only",
			systemPrompt:   "You are a coding assistant",
			wantContains:   []string{"exec", "--json", "-c", "developer_instructions=You are a coding assistant"},
			wantNotContain: []string{"--model", "--mcp-config"},
		},
		{
			name:           "with all fields set",
			model:          "gpt-4o",
			workDir:        "/tmp/work",
			systemPrompt:   "You are a coding assistant",
			wantContains:   []string{"exec", "--json", "--model", "gpt-4o", "-c", "developer_instructions=You are a coding assistant"},
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
			wantContains:   []string{"exec", "--json", "--mcp-config"},
			wantNotContain: []string{"--model", "-c"},
		},
		{
			name:           "with nil mcp config",
			model:          "gpt-4o",
			workDir:        "/tmp/work",
			wantContains:   []string{"exec", "--json", "--model", "gpt-4o"},
			wantNotContain: []string{"--mcp-config", "-c"},
		},
		{
			name:    "with mcp config but no servers",
			workDir: "/tmp/work",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{},
			},
			wantContains:   []string{"exec", "--json"},
			wantNotContain: []string{"--mcp-config", "--model", "-c"},
		},
		{
			name:    "with mcp config but empty workDir",
			workDir: "",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{
					{Name: "test-server", Command: "echo"},
				},
			},
			wantContains:   []string{"exec", "--json"},
			wantNotContain: []string{"--mcp-config", "--model", "-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := a.buildArgs(tt.model, tt.workDir, tt.mcp, tt.systemPrompt)
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
				SystemPrompt: "You are a coding agent",
			},
			want: "You are a coding agent\n\n",
		},
		{
			name: "messages only",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: "Write a function"},
					{Content: "Test it"},
				},
			},
			want: "Write a function\nTest it\n",
		},
		{
			name: "system prompt and messages",
			req: unified.ExecuteRequest{
				SystemPrompt: "You are a coding agent",
				Messages: []*schema.Message{
					{Content: "Refactor this code"},
				},
			},
			want: "You are a coding agent\n\nRefactor this code\n",
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
		t.Error("Codex CLI should support tools")
	}
	if !caps1.SupportsFileOps {
		t.Error("Codex CLI should support file ops")
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
	a.BinaryPath = "codex-not-installed"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Health(ctx); err == nil {
		t.Error("expected error for missing binary even with cancelled context")
	}
}

func TestParseCodexEvent(t *testing.T) {
	tests := []struct {
		name        string
		obj         map[string]any
		wantType    string
		wantContent string
	}{
		{
			name:        "message event",
			obj:         map[string]any{"type": "message", "content": "Hello world"},
			wantType:    "text",
			wantContent: "Hello world",
		},
		{
			name:        "function_call event",
			obj:         map[string]any{"type": "function_call", "name": "shell"},
			wantType:    "tool_call",
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
			name:        "message missing content",
			obj:         map[string]any{"type": "message"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "message non-string content",
			obj:         map[string]any{"type": "message", "content": 42},
			wantType:    "text",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := parseCodexEvent(tt.obj)
			if evt.Type != tt.wantType {
				t.Errorf("type = %q, want %q", evt.Type, tt.wantType)
			}
			if evt.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", evt.Content, tt.wantContent)
			}
		})
	}
}
