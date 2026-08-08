package opencode

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
	if caps.OutputFormat != "ndjson" {
		t.Errorf("output format = %s, want ndjson", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "opencode-not-installed"
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
	if a.BinaryPath != "opencode" {
		t.Errorf("BinaryPath = %q, want %q", a.BinaryPath, "opencode")
	}
}

func TestAdapterBuildArgsTableDriven(t *testing.T) {
	a := New(nil)
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		workDir        string
		mcp            *unified.MCPConfig
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "all empty",
			wantContains:   []string{"--output", "ndjson"},
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
			wantContains:   []string{"--output", "ndjson", "--mcp-config"},
			wantNotContain: []string{},
		},
		{
			name:           "with nil mcp config",
			workDir:        "/tmp/work",
			wantContains:   []string{"--output", "ndjson"},
			wantNotContain: []string{"--mcp-config"},
		},
		{
			name:    "with mcp config but no servers",
			workDir: "/tmp/work",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{},
			},
			wantContains:   []string{"--output", "ndjson"},
			wantNotContain: []string{"--mcp-config"},
		},
		{
			name:    "with mcp config but empty workDir",
			workDir: "",
			mcp: &unified.MCPConfig{
				Servers: []unified.MCPServer{
					{Name: "test-server", Command: "echo"},
				},
			},
			wantContains:   []string{"--output", "ndjson"},
			wantNotContain: []string{"--mcp-config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := a.buildArgs(tt.workDir, tt.mcp)
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
				SystemPrompt: "You are a guard dog",
			},
			want: "You are a guard dog\n\n",
		},
		{
			name: "messages only",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: "Validate input"},
					{Content: "Check paths"},
				},
			},
			want: "Validate input\nCheck paths\n",
		},
		{
			name: "system prompt and messages",
			req: unified.ExecuteRequest{
				SystemPrompt: "You are a guard dog",
				Messages: []*schema.Message{
					{Content: "Filter sensitive data"},
				},
			},
			want: "You are a guard dog\n\nFilter sensitive data\n",
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
	if !caps1.SupportsMCP {
		t.Error("opencode CLI should support MCP")
	}
	if !caps1.SupportsTools {
		t.Error("opencode CLI should support tools")
	}
	if !caps1.SupportsFileOps {
		t.Error("opencode CLI should support file ops")
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
	a.BinaryPath = "opencode-not-installed"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Health(ctx); err == nil {
		t.Error("expected error for missing binary even with cancelled context")
	}
}

func TestParseOpencodeEvent(t *testing.T) {
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
			name:        "tool event",
			obj:         map[string]any{"type": "tool", "name": "edit"},
			wantType:    "tool_call",
			wantContent: "",
		},
		{
			name:        "session event",
			obj:         map[string]any{"type": "session", "id": "abc123"},
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
			name:        "message missing content",
			obj:         map[string]any{"type": "message"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "message non-string content",
			obj:         map[string]any{"type": "message", "content": true},
			wantType:    "text",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := parseOpencodeEvent(tt.obj)
			if evt.Type != tt.wantType {
				t.Errorf("type = %q, want %q", evt.Type, tt.wantType)
			}
			if evt.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", evt.Content, tt.wantContent)
			}
		})
	}
}
