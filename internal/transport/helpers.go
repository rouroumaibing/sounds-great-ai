package transport

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

func (h *WSHandler) sendBarkError(sessionID, breedID, errMsg string) {
	h.mu.RLock()
	s := h.streamers[sessionID]
	h.mu.RUnlock()
	if s != nil {
		s.SendEvent(context.Background(), protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
			Breed: breedID, Error: errMsg,
		}))
	}
}

func (h *WSHandler) countToolCalls(sessionID string) int {
	if h.platform == nil || h.platform.MessageStore == nil {
		return 0
	}
	history, _ := h.platform.MessageStore.GetByThread(sessionID, 100)
	count := 0
	for _, msg := range history {
		if msg.Role == "assistant" {
			count++
		}
	}
	return count
}

func (h *WSHandler) populateLeaderContext(input *hooks.AssemblerInput) {
	if h.platform == nil || h.platform.Leader == nil {
		return
	}
	leader := h.platform.Leader
	input.LeaderName = leader.Name
	input.LeaderFirstMention = leader.MentionPatterns[0]
	parts := make([]string, len(leader.MentionPatterns))
	for i, p := range leader.MentionPatterns {
		parts[i] = "`" + p + "`"
	}
	input.LeaderHandles = strings.Join(parts, " / ")
}

func (h *WSHandler) injectHooks(basePrompt, breedID, breedName, roleDesc, personality, query, sessionID string) (string, string) {
	systemPrompt := basePrompt
	systemPromptL0 := ""
	if h.platform.HookPipeline == nil {
		return systemPrompt, systemPromptL0
	}
	// G5：解析当前执行目标 carrier（variant.ClientID），注入 AssemblerInput 供
	// skill-trigger resolver 按挂载范围过滤。未知 carrier 时置空，resolver 降级为不过滤。
	carrier := ""
	if b := h.platform.GetBreed(breedID); b != nil {
		if v := b.DefaultVariant(); v != nil {
			carrier = v.ClientID
		}
	}
	hookInput := &hooks.AssemblerInput{
		BreedID: breedID, BreedName: breedName, RoleDescription: roleDesc,
		Personality: personality, CurrentPhase: currentPhase(),
		ToolCallCount: h.countToolCalls(sessionID), TaskID: sessionID, Query: query,
		Carrier: carrier,
	}
	h.populateLeaderContext(hookInput)
	initResult := h.platform.HookPipeline.ExecuteStage("session-init", hookInput)
	initPrompt := hooks.AssemblePatches(initResult.Patches)
	turnResult := h.platform.HookPipeline.ExecuteStage("per-turn", hookInput)
	turnPrompt := hooks.AssemblePatches(turnResult.Patches)
	breed := h.platform.GetBreed(breedID)
	variant := breed.DefaultVariant()
	if variant != nil && h.supportsNativeL0(variant) {
		systemPromptL0 = initPrompt
		if turnPrompt != "" {
			systemPrompt = turnPrompt + "\n\n" + systemPrompt
		}
	} else {
		var hookPrompt string
		if initPrompt != "" && turnPrompt != "" {
			hookPrompt = initPrompt + "\n\n" + turnPrompt
		} else if initPrompt != "" {
			hookPrompt = initPrompt
		} else {
			hookPrompt = turnPrompt
		}
		if hookPrompt != "" {
			systemPrompt = hookPrompt + "\n\n" + systemPrompt
		}
	}
	if h.platform.HookTraceStore != nil {
		allEvents := append(initResult.Events, turnResult.Events...)
		if len(allEvents) > 0 {
			turnID := time.Now().Format(time.RFC3339Nano)
			if err := h.platform.HookTraceStore.Persist(sessionID, turnID, allEvents); err != nil {
				fmt.Printf("warning: hook trace persist failed: %v\n", err)
			}
		}
	}
	return systemPrompt, systemPromptL0
}

func currentPhase() string {
	phase := os.Getenv("CURRENT_PHASE")
	if phase == "" {
		return "7"
	}
	return phase
}

func supportsNativeL0(cliCommand string) bool {
	return cliCommand == "claude" || cliCommand == "codex"
}

// supportsNativeL0 resolves whether a breed's CLI supports a native,
// compression-immune L0 system-prompt channel (G6). It is data-driven: the
// decision comes from the registered adapter's AgentCapabilities (set per
// provider), falling back to the legacy command-whitelist only if the executor
// is unavailable. Keeping this in the adapter capabilities avoids hard-coding
// CLI command strings in the transport layer.
func (h *WSHandler) supportsNativeL0(variant *pack.Variant) bool {
	if variant == nil {
		return false
	}
	if h.platform != nil && h.platform.AgentExecutor != nil {
		if h.platform.AgentExecutor.Capabilities(variant.ClientID).SupportsNativeL0 {
			return true
		}
	}
	return supportsNativeL0(variant.CLI.Command)
}

func convertStreamEvent(ev unified.StreamEvent, sessionID, breedID string) *protocol.Event {
	switch ev.Type {
	case "thinking":
		return protocol.NewEvent(protocol.EventThinking, sessionID, &protocol.ThinkingPayload{Step: 1, Content: ev.Content})
	case "text":
		// G1: stream assistant text deltas live instead of dropping them. The
		// frontend accumulates these into a running breed_response block.
		return protocol.NewEvent(protocol.EventAgentMessage, sessionID, &protocol.AgentMessagePayload{
			Breed: breedID, Content: ev.Content, Done: false,
		})
	case "tool_call":
		toolName, _ := ev.Meta["tool"].(string)
		return protocol.NewEvent(protocol.EventToolCall, sessionID, &protocol.ToolCallPayload{Tool: toolName, Params: ev.Content})
	case "error":
		// Surface structured diagnostics (cliDiagnostics) when
		// the adapter populated StreamEvent.Meta. The server has already
		// sanitized the excerpt (REDACTED-*) and classified the reason; the
		// client additionally gates raw excerpt display by Source allowlist.
		meta := map[string]string{}
		if raw, ok := ev.Meta["meta"].(map[string]any); ok {
			for k, v := range raw {
				if s, ok := v.(string); ok {
					meta[k] = s
				}
			}
		}
		str := func(k string) string {
			if v, ok := ev.Meta[k].(string); ok {
				return v
			}
			return ""
		}
		return protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
			Breed:   breedID,
			Error:   ev.Content,
			Reason:  str("reason"),
			Summary: str("summary"),
			Hint:    str("hint"),
			Excerpt: str("excerpt"),
			Source:  str("source"),
			Meta:    meta,
		})
	case "stall_warning":
		// R8: forward liveness-probe state changes to the client so a stalled
		// (alive-but-silent) CLI is visible instead of failing silently.
		state, _ := ev.Meta["state"].(string)
		hard, _ := ev.Meta["hard"].(bool)
		return protocol.NewEvent(protocol.EventAgentLiveness, sessionID, &protocol.LivenessPayload{
			Breed:   breedID,
			State:   state,
			Hard:    hard,
			Message: ev.Content,
		})
	default:
		return nil
	}
}

func ragEnabled() bool {
	v := os.Getenv("RAG_ENABLED")
	if v == "" {
		return true
	}
	return v == "true" || v == "1" || v == "yes"
}

func breedHasRetrieverRole(breed *pack.BreedConfig) bool {
	if breed == nil {
		return false
	}
	for _, role := range breed.Roles {
		if role == "retriever" {
			return true
		}
	}
	return false
}

func (h *WSHandler) retrieveRAGContext(ctx context.Context, breedID, query string) string {
	if !ragEnabled() || h.platform == nil || h.platform.RAGRegistry == nil {
		return ""
	}
	breed := h.platform.GetBreed(breedID)
	if !breedHasRetrieverRole(breed) {
		return ""
	}
	retrieveCap := capability.NewRetrieveCapability(h.platform.RAGRegistry, h.platform.Embedder)
	retrieveOut, err := retrieveCap.Run(ctx, &pack.TaskInput{Query: query})
	if err != nil || retrieveOut == nil {
		return ""
	}
	matches, _ := retrieveOut.Data["matches"].([]any)
	if len(matches) == 0 {
		return ""
	}
	assembleCap := capability.NewContextAssemble()
	assembleOut, err := assembleCap.Run(ctx, &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{"retrieve": retrieveOut},
	})
	if err != nil || assembleOut == nil {
		return ""
	}
	context, _ := assembleOut.Data["context"].(string)
	return context
}

func (h *WSHandler) GetStreamer(sessionID string) *Streamer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.streamers[sessionID]
}

func (h *WSHandler) SessionCount() int {
	return h.rateMonitor.SessionCount()
}

func (h *WSHandler) SendSystemNotice(severity, title, message string) {
	notice := protocol.SystemNoticePayload{
		Severity:  severity,
		Title:     title,
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	event := protocol.NewEvent(protocol.EventSystemNotice, "", &notice)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, s := range h.streamers {
		s.SendEvent(context.Background(), event)
	}
}

// emitSopGate sends a SOP_GATE event for the given session so the frontend can
// render the cross-breed review routing / gate status.
func (h *WSHandler) emitSopGate(ctx context.Context, sessionID, author, reviewer, reason string, blocked bool) {
	event := protocol.NewEvent(protocol.EventSopGate, sessionID, &protocol.SopGatePayload{
		Reason:   reason,
		Author:   author,
		Reviewer: reviewer,
		Blocked:  blocked,
	})
	if s := h.GetStreamer(sessionID); s != nil {
		_ = s.SendEvent(ctx, event)
	}
}
