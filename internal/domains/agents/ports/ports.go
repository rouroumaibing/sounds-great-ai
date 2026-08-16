// Package ports defines the agents domain port: the interface through which
// the transport layer invokes a breed's CLI agent (Claude/Codex/Gemini/opencode)
// without depending on the concrete adapters in internal/adapter.
//
// This is Phase E (D4-3) of the orchestration refactor: the four CLI adapters
// are wrapped behind this port so execution.go consumes a domain abstraction
// rather than reaching into internal/adapter directly (spec R1: ExecuteRequest
// is expanded with Variant/ClientID/ThreadID/SessionID/Context).
package ports

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/adapter/unified"
)

// StreamEvent aliases the unified stream event so transport can consume the
// invocation stream without importing internal/adapter directly.
type StreamEvent = unified.StreamEvent

// ExecuteRequest is the domain-level request to invoke a single breed's CLI
// agent. It expands unified.ExecuteRequest (spec R1) with the resolved
// ClientID, routing context (ThreadID/SessionID), and the caller's context
// for tracing — keeping the concrete adapter type out of the transport layer.
type ExecuteRequest struct {
	// ClientID selects the underlying CLI adapter (claude/codex/gemini/opencode).
	ClientID string
	// Messages is the conversation history in Eino schema format.
	Messages []*schema.Message
	// SystemPrompt is the breed persona prompt (+ injected skills).
	SystemPrompt string
	// SystemPromptL0 is the native L0 system prompt (compression-immune).
	SystemPromptL0 string
	// Model is the model variant (e.g. "claude-opus-4-6").
	Model string
	// Skills are skill prompt IDs to inject.
	Skills []string
	// MCPConfig carries MCP server configs for the CLI.
	MCPConfig *unified.MCPConfig
	// WorkDir is the working directory for file ops.
	WorkDir string
	// MaxTokens is the response budget (0 = CLI default).
	MaxTokens int
	// AutoCompactTokenLimit is the breed's configured context-compaction budget
	// (Persistent Identity P2, homologous auto-compact). When > 0 the
	// orchestration bounds the assembled history to this token budget and the
	// transport forwards it to the CLI process env. 0 = platform default.
	AutoCompactTokenLimit int
	// ThreadID / SessionID identify the orchestration thread (ball-custody).
	ThreadID  string
	SessionID string
	// Context carries the caller's context for tracing/telemetry. If nil, the
	// Execute ctx is used.
	Context context.Context
}

// IAgentExecutor is the agents domain port: one breed's CLI invocation.
type IAgentExecutor interface {
	// Execute sends a message to the CLI agent selected by req.ClientID and
	// returns a stream of events.
	Execute(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error)
	// Capabilities returns what the given CLI backend supports.
	Capabilities(clientID string) unified.AgentCapabilities
	// Health checks if the CLI binary for clientID is available/authenticated.
	Health(ctx context.Context, clientID string) error
	// Get returns the underlying CLI adapter (used by eval + tests that need
	// the concrete unified.AgentExecutor).
	Get(clientID string) (unified.AgentExecutor, error)
	// Count returns the number of configured CLI adapters.
	Count() int
}
