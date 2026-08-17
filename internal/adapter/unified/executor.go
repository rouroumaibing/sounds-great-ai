package unified

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// AgentExecutor is the unified interface all CLI adapters implement.
type AgentExecutor interface {
	// Execute sends a message to the CLI agent and returns a stream of events.
	Execute(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error)

	// Capabilities returns what this CLI supports.
	Capabilities() AgentCapabilities

	// Health checks if the CLI binary is available and authenticated.
	Health(ctx context.Context) error
}

// ExecuteRequest carries all information needed to invoke a CLI agent.
type ExecuteRequest struct {
	Messages       []*schema.Message // Eino message format (conversation history)
	SystemPrompt   string            // Breed persona prompt + injected skills
	SystemPromptL0 string            // Native L0 system prompt (compression-immune, via CLI flag)
	// ClientID selects the underlying adapter (claude/codex/gemini/opencode/kimi
	// or the a2a protocol client). The domain layer populates it; the unified
	// adapters that don't care (CLI) ignore it, while the A2A adapter uses it to
	// resolve the external endpoint.
	ClientID       string
	Model          string            // Model variant (e.g., "claude-opus-4-6")
	Skills         []string          // Skill prompt IDs to inject
	MCPConfig      *MCPConfig        // MCP server configs to pass to CLI
	WorkDir        string            // Working directory for file ops
	MaxTokens      int               // Response budget (0 = CLI default)
	// AutoCompactTokenLimit is the breed's configured context-compaction budget
	// (Persistent Identity P2, CLI-native auto-compact). When
	// > 0 the transport/adapter forwards it to the CLI so the CLI itself
	// compresses in-session (the CLI injects `--config=model_auto_compact_token_limit=<N>`
	// for codex; claude/gemini rely on CLI-native autoCompact and need no flag).
	// 0 = platform default (the orchestration still bounds history at the
	// platform level via this same value).
	AutoCompactTokenLimit int
}

// AgentCapabilities describes what a CLI backend supports.
type AgentCapabilities struct {
	SupportsMCP      bool
	SupportsTools    bool
	SupportsFileOps  bool
	OutputFormat     string // "stream-json" | "json" | "ndjson"
	SupportsNativeL0 bool   // native, compression-immune L0 system-prompt channel
}

// MCPConfig holds MCP server configurations to pass to the CLI.
type MCPConfig struct {
	Servers []MCPServer `json:"servers"`
}

// MCPServer is one MCP server entry.
type MCPServer struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}
