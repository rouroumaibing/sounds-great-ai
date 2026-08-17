// Package services implements the agents domain port by wrapping the concrete
// CLI adapters (claude/codex/gemini/opencode) registered in the platform.
package services

import (
	"context"
	"fmt"

	"sounds-great-ai/internal/adapter/unified"
	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
)

// AgentExecutorService is the agents domain service: it resolves a ClientID to
// the registered CLI adapter and delegates invocation through it.
type AgentExecutorService struct {
	adapters map[string]unified.AgentExecutor
}

// NewAgentExecutor wraps a map of CLI adapters (keyed by CLI name) behind the
// agents port.
func NewAgentExecutor(adapters map[string]unified.AgentExecutor) *AgentExecutorService {
	return &AgentExecutorService{adapters: adapters}
}

// Execute resolves req.ClientID to the underlying adapter and delegates. The
// caller's context (req.Context, falling back to ctx) drives tracing.
func (s *AgentExecutorService) Execute(ctx context.Context, req agentsPorts.ExecuteRequest) (<-chan agentsPorts.StreamEvent, error) {
	a, err := s.Get(req.ClientID)
	if err != nil {
		return nil, err
	}
	callCtx := req.Context
	if callCtx == nil {
		callCtx = ctx
	}
	uReq := unified.ExecuteRequest{
		Messages:              req.Messages,
		SystemPrompt:          req.SystemPrompt,
		SystemPromptL0:        req.SystemPromptL0,
		ClientID:              req.ClientID,
		Model:                 req.Model,
		Skills:                req.Skills,
		MCPConfig:             req.MCPConfig,
		WorkDir:               req.WorkDir,
		MaxTokens:             req.MaxTokens,
		AutoCompactTokenLimit: req.AutoCompactTokenLimit,
	}
	return a.Execute(callCtx, uReq)
}

// Capabilities returns what the selected CLI backend supports.
func (s *AgentExecutorService) Capabilities(clientID string) unified.AgentCapabilities {
	a, err := s.Get(clientID)
	if err != nil {
		return unified.AgentCapabilities{}
	}
	return a.Capabilities()
}

// Health checks the CLI binary availability for clientID.
func (s *AgentExecutorService) Health(ctx context.Context, clientID string) error {
	a, err := s.Get(clientID)
	if err != nil {
		return err
	}
	return a.Health(ctx)
}

// Get returns the underlying CLI adapter by name.
func (s *AgentExecutorService) Get(clientID string) (unified.AgentExecutor, error) {
	a, ok := s.adapters[clientID]
	if !ok {
		return nil, fmt.Errorf("unknown CLI: %s", clientID)
	}
	return a, nil
}

// Count returns the number of registered CLI adapters.
func (s *AgentExecutorService) Count() int { return len(s.adapters) }

// Ensure AgentExecutorService satisfies the port at compile time.
var _ agentsPorts.IAgentExecutor = (*AgentExecutorService)(nil)
