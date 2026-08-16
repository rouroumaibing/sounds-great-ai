package platform

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	"sounds-great-ai/internal/domains/agents/ports"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/pkg/pack"
)

// peopleMemoryClerkDeps wires the daily people-memory clerk to the platform's
// dog-invocation path so the clerk can re-invoke the ORIGINAL dog (F276
// parity): the dog re-derives the proposal from the exact sources; the
// platform only persists the returned, rejectable candidate. Reasoning stays
// in the CLI adapter (AGENTS.md §3 three-layer discipline) — the platform never
// reasons over memory itself.
func (p *Platform) peopleMemoryClerkDeps() settings.PeopleMemoryClerkDeps {
	invoke := func(ctx context.Context, clientID, promptText, workDir string) (string, error) {
		if p.AgentExecutor == nil {
			return "", fmt.Errorf("no agent executor configured")
		}
		breed := p.resolveBreedByClientID(clientID)
		if breed == nil {
			return "", fmt.Errorf("no breed configured for client %q", clientID)
		}
		variant := breed.DefaultVariant()
		systemPrompt := ""
		if p.PromptBuilder != nil && variant != nil {
			systemPrompt = p.PromptBuilder.Build(prompt.BuildRequest{BreedID: breed.ID, VariantID: variant.ID})
		}
		req := ports.ExecuteRequest{
			ClientID:     clientID,
			Messages:     []*schema.Message{schema.UserMessage(promptText)},
			SystemPrompt: systemPrompt,
			Model:        variant.DefaultModel,
			WorkDir:      workDir,
			MCPConfig:    p.BuildMCPConfig(),
			ThreadID:     "pm-clerk",
			SessionID:    "pm-clerk",
			Context:      ctx,
		}
		ch, err := p.AgentExecutor.Execute(ctx, req)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for ev := range ch {
			if ev.Type == "text" {
				sb.WriteString(ev.Content)
			}
			if ev.IsError() {
				return sb.String(), fmt.Errorf("dog returned error: %s", ev.Content)
			}
		}
		return sb.String(), nil
	}

	resolveSource := func(_ string, ref settings.SourceRef) (string, bool) {
		if p.MessageStore == nil || ref.ThreadID == "" || ref.MessageID == "" {
			return "", false
		}
		msgs, err := p.MessageStore.GetByThread(ref.ThreadID, 200)
		if err != nil || len(msgs) == 0 {
			return "", false
		}
		for _, m := range msgs {
			if m.ID == ref.MessageID {
				return m.Content, true
			}
		}
		return "", false
	}

	return settings.PeopleMemoryClerkDeps{
		Invoke:          invoke,
		ResolveSource:   resolveSource,
		DefaultClientID: p.defaultPeopleMemoryClientID(),
		WorkDir:         p.WorkspaceDir,
	}
}

// resolveBreedByClientID returns the first breed whose default variant uses the
// given CLI client id (claude/codex/gemini/opencode/kimi).
func (p *Platform) resolveBreedByClientID(clientID string) *pack.BreedConfig {
	for _, b := range p.Breeds {
		if v := b.DefaultVariant(); v != nil && strings.EqualFold(v.ClientID, clientID) {
			return b
		}
	}
	return nil
}

// defaultPeopleMemoryClientID returns a usable client id for the clerk when a
// deferred receipt carries none. It is the first configured breed's client id.
func (p *Platform) defaultPeopleMemoryClientID() string {
	for _, b := range p.Breeds {
		if v := b.DefaultVariant(); v != nil && strings.TrimSpace(v.ClientID) != "" {
			return v.ClientID
		}
	}
	return ""
}
