package gemini

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
	if caps.OutputFormat != "stream-json" {
		t.Errorf("output format = %s, want stream-json", caps.OutputFormat)
	}
}

func TestAdapterHealthMissingBinary(t *testing.T) {
	a := New(nil)
	a.BinaryPath = "gemini-not-installed"
	if err := a.Health(context.Background()); err == nil {
		t.Error("expected error for missing binary")
	}
}

// --- New edge case and error case tests ---

func TestAdapterNewDefaults(t *testing.T) {
	a := New(nil)
	if a.BinaryPath != "gemini" {
		t.Errorf("BinaryPath = %q, want %q", a.BinaryPath, "gemini")
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
				SystemPrompt: "You are a retriever",
			},
			want: "You are a retriever\n\n",
		},
		{
			name: "messages only",
			req: unified.ExecuteRequest{
				Messages: []*schema.Message{
					{Content: "Search for X"},
					{Content: "Summarize results"},
				},
			},
			want: "Search for X\nSummarize results\n",
		},
		{
			name: "system prompt and messages",
			req: unified.ExecuteRequest{
				SystemPrompt: "You are a retriever",
				Messages: []*schema.Message{
					{Content: "Find relevant docs"},
				},
			},
			want: "You are a retriever\n\nFind relevant docs\n",
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
	if caps1.SupportsMCP {
		t.Error("Gemini CLI should not support MCP")
	}
	if !caps1.SupportsTools {
		t.Error("Gemini CLI should support tools")
	}
	if !caps1.SupportsFileOps {
		t.Error("Gemini CLI should support file ops")
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
	a.BinaryPath = "gemini-not-installed"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Health(ctx); err == nil {
		t.Error("expected error for missing binary even with cancelled context")
	}
}

func TestParseGeminiEvent(t *testing.T) {
	tests := []struct {
		name        string
		obj         map[string]any
		wantType    string
		wantContent string
	}{
		{
			name:        "text event",
			obj:         map[string]any{"type": "text", "content": "Hello world"},
			wantType:    "text",
			wantContent: "Hello world",
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
			name:        "text missing content",
			obj:         map[string]any{"type": "text"},
			wantType:    "text",
			wantContent: "",
		},
		{
			name:        "text non-string content",
			obj:         map[string]any{"type": "text", "content": 3.14},
			wantType:    "text",
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := parseGeminiEvent(tt.obj)
			if evt.Type != tt.wantType {
				t.Errorf("type = %q, want %q", evt.Type, tt.wantType)
			}
			if evt.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", evt.Content, tt.wantContent)
			}
		})
	}
}
