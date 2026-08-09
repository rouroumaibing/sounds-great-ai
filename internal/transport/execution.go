package transport

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/pkg/protocol"

	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func (h *WSHandler) executeWithPlatform(ctx context.Context, breedID, sessionID, query string) string {
	h.mu.RLock()
	streamer := h.streamers[sessionID]
	h.mu.RUnlock()
	if streamer == nil {
		return ""
	}

	breed := h.platform.GetBreed(breedID)
	if breed == nil {
		h.sendBarkError(sessionID, breedID, fmt.Sprintf("breed %q not found in platform", breedID))
		return ""
	}
	variant := breed.DefaultVariant()
	if variant == nil {
		h.sendBarkError(sessionID, breedID, "no variant configured")
		return ""
	}
	adapter, err := h.platform.GetAdapter(variant.ClientID)
	if err != nil {
		h.sendBarkError(sessionID, breedID, fmt.Sprintf("adapter %q not available: %v", variant.ClientID, err))
		return ""
	}

	systemPrompt := variant.SystemPrompt
	ragContext := h.retrieveRAGContext(ctx, breedID, query)
	if h.platform.PromptBuilder != nil {
		systemPrompt = h.platform.PromptBuilder.Build(prompt.BuildRequest{
			BreedID: breedID, VariantID: variant.ID, RAGContext: ragContext,
		})
	}
	systemPrompt, systemPromptL0 := h.injectHooks(systemPrompt, breedID, breed.DisplayName, breed.RoleDescription, breed.Personality, sessionID)

	var messages []*schema.Message
	if h.platform.MessageStore != nil {
		sender := ""
		if h.platform.Leader != nil && isLeaderMention(query, h.platform.Leader.MentionPatterns) {
			sender = "leader"
		}
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID: sessionID, Role: "user", Content: query, Sender: sender, Timestamp: time.Now(),
		})
		history, _ := h.platform.MessageStore.GetByThread(sessionID, 21)
		var contextMsgs []prompt.ContextMessage
		for _, msg := range history {
			if msg.Content == query && msg.Role == "user" {
				continue
			}
			contextMsgs = append(contextMsgs, prompt.ContextMessage{
				Role: msg.Role, Content: msg.Content, Sender: msg.Sender, Timestamp: msg.Timestamp,
			})
		}
		messages = prompt.ToSchemaMessages(contextMsgs)
	}
	messages = append(messages, schema.UserMessage(query))

	req := unified.ExecuteRequest{
		Messages: messages, SystemPrompt: systemPrompt, SystemPromptL0: systemPromptL0,
		Model: variant.DefaultModel, WorkDir: h.platform.WorkspaceDir, MCPConfig: h.platform.BuildMCPConfig(),
	}

	execCtx := ctx
	var span trace.Span
	execStart := time.Now()
	if telemetry.IsInitialized() {
		tracer := otel.Tracer("sounds-great-ai")
		execCtx, span = tracer.Start(ctx, "breed.execute")
		span.SetAttributes(attribute.String("breed", breedID))
	}

	eventCh, err := adapter.Execute(execCtx, req)
	if err != nil {
		if span != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
		}
		if telemetry.IsInitialized() && telemetry.InvocationCompleted != nil {
			telemetry.InvocationCompleted.Add(execCtx, 1, metric.WithAttributes(
				attribute.String("breed", breedID), attribute.String("status", "error")))
		}
		h.sendBarkError(sessionID, breedID, fmt.Sprintf("execute failed: %v", err))
		return ""
	}

	var responseText strings.Builder
	for ev := range eventCh {
		if ev.Type == "text" {
			responseText.WriteString(ev.Content)
		}
		protoEvent := convertStreamEvent(ev, sessionID, breedID)
		if protoEvent != nil {
			h.rateMonitor.Record(sessionID)
			streamer.SendEvent(ctx, protoEvent)
		}
	}

	if span != nil {
		span.End()
	}
	if telemetry.IsInitialized() {
		duration := time.Since(execStart)
		if telemetry.InvocationDuration != nil {
			telemetry.InvocationDuration.Record(execCtx, duration.Milliseconds(),
				metric.WithAttributes(attribute.String("breed", breedID)))
		}
		if telemetry.InvocationCompleted != nil {
			telemetry.InvocationCompleted.Add(execCtx, 1, metric.WithAttributes(
				attribute.String("breed", breedID), attribute.String("status", "ok")))
		}
		if ts := telemetry.TraceStoreInstance(); ts != nil {
			s := telemetry.Span{
				Name: "breed.execute", StartTime: execStart, EndTime: time.Now(),
				Attributes: map[string]any{"breed": breedID, "duration": duration.Milliseconds()},
				Status: "ok",
			}
			if r := telemetry.RedactorInstance(); r != nil {
				r.RedactSpan(&s)
			}
			ts.Add(s)
		}
	}

	if h.platform.MessageStore != nil && responseText.Len() > 0 {
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID: sessionID, Role: "assistant", Content: responseText.String(), Sender: breedID, Timestamp: time.Now(),
		})
	}

	resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
		Breed: breedID, Success: true, Steps: make(map[string]protocol.StepResult),
	})
	streamer.SendEvent(ctx, resultEvent)

	if h.platform != nil && h.platform.A2AHub != nil && h.platform.SOP != nil {
		mentions := detectMentionInResponse(responseText.String())
		for _, toBreed := range mentions {
			if toBreed == breedID || h.platform.GetBreed(toBreed) == nil {
				continue
			}
			thread := h.platform.A2AHub.GetThread(sessionID)
			if thread == nil {
				thread = h.platform.A2AHub.CreateThread(query, []string{breedID})
			}
			h.handleA2AHandoff(ctx, thread, breedID, toBreed, sessionID, responseText.String())
		}
	}

	return responseText.String()
}

func (h *WSHandler) executeSerial(ctx context.Context, breedIDs []string, sessionID, query string) {
	previousOutput := ""
	for i, breedID := range breedIDs {
		currentQuery := query
		if i > 0 && previousOutput != "" {
			currentQuery = fmt.Sprintf("%s\n\n[%s 的输出]:\n%s", query, breedIDs[i-1], previousOutput)
		}
		h.mu.RLock()
		streamer := h.streamers[sessionID]
		h.mu.RUnlock()
		if streamer != nil {
			streamer.SendEvent(ctx, protocol.NewEvent(protocol.EventBarkStart, sessionID, &protocol.BarkStartPayload{
				Breed: breedID, SessionID: sessionID, Query: currentQuery,
			}))
		}
		previousOutput = h.executeWithPlatform(ctx, breedID, sessionID, currentQuery)
	}
}

func (h *WSHandler) executeParallel(ctx context.Context, breedIDs []string, sessionID, query string) {
	if h.platform.MessageStore != nil {
		sender := ""
		if h.platform.Leader != nil && isLeaderMention(query, h.platform.Leader.MentionPatterns) {
			sender = "leader"
		}
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID: sessionID, Role: "user", Content: query, Sender: sender, Timestamp: time.Now(),
		})
	}
	var history []*threadstore.Message
	if h.platform.MessageStore != nil {
		history, _ = h.platform.MessageStore.GetByThread(sessionID, 21)
	}
	var contextMsgs []prompt.ContextMessage
	for _, msg := range history {
		if msg.Content == query && msg.Role == "user" {
			continue
		}
		contextMsgs = append(contextMsgs, prompt.ContextMessage{
			Role: msg.Role, Content: msg.Content, Sender: msg.Sender, Timestamp: msg.Timestamp,
		})
	}
	sharedSchemaMsgs := prompt.ToSchemaMessages(contextMsgs)
	sharedSchemaMsgs = append(sharedSchemaMsgs, schema.UserMessage(query))

	var wg sync.WaitGroup
	responses := make([]struct{ breedID, text string }, len(breedIDs))
	for i, breedID := range breedIDs {
		wg.Add(1)
		go func(idx int, bid string) {
			defer wg.Done()
			h.mu.RLock()
			streamer := h.streamers[sessionID]
			h.mu.RUnlock()
			if streamer == nil {
				return
			}
			breed := h.platform.GetBreed(bid)
			if breed == nil {
				h.sendBarkError(sessionID, bid, fmt.Sprintf("breed %q not found", bid))
				return
			}
			variant := breed.DefaultVariant()
			if variant == nil {
				h.sendBarkError(sessionID, bid, "no variant configured")
				return
			}
			adapter, err := h.platform.GetAdapter(variant.ClientID)
			if err != nil {
				h.sendBarkError(sessionID, bid, fmt.Sprintf("adapter %q not available: %v", variant.ClientID, err))
				return
			}
			systemPrompt := variant.SystemPrompt
			ragContext := h.retrieveRAGContext(ctx, bid, query)
			if h.platform.PromptBuilder != nil {
				systemPrompt = h.platform.PromptBuilder.Build(prompt.BuildRequest{
					BreedID: bid, VariantID: variant.ID, RAGContext: ragContext,
				})
			}
			systemPrompt, systemPromptL0 := h.injectHooks(systemPrompt, bid, breed.DisplayName, breed.RoleDescription, breed.Personality, sessionID)
			req := unified.ExecuteRequest{
				Messages: sharedSchemaMsgs, SystemPrompt: systemPrompt, SystemPromptL0: systemPromptL0,
				Model: variant.DefaultModel, WorkDir: h.platform.WorkspaceDir, MCPConfig: h.platform.BuildMCPConfig(),
			}
			eventCh, err := adapter.Execute(ctx, req)
			if err != nil {
				h.sendBarkError(sessionID, bid, fmt.Sprintf("execute failed: %v", err))
				return
			}
			var respText strings.Builder
			for ev := range eventCh {
				if ev.Type == "text" {
					respText.WriteString(ev.Content)
				}
				protoEvent := convertStreamEvent(ev, sessionID, bid)
				if protoEvent != nil {
					h.rateMonitor.Record(sessionID)
					streamer.SendEvent(ctx, protoEvent)
				}
			}
			responses[idx].breedID = bid
			responses[idx].text = respText.String()
		}(i, breedID)
	}
	wg.Wait()

	if h.platform.MessageStore != nil {
		for _, resp := range responses {
			if resp.text != "" {
				h.platform.MessageStore.Append(&threadstore.Message{
					ThreadID: sessionID, Role: "assistant", Content: resp.text, Sender: resp.breedID, Timestamp: time.Now(),
				})
			}
		}
	}
	h.mu.RLock()
	streamer := h.streamers[sessionID]
	h.mu.RUnlock()
	if streamer != nil {
		streamer.SendEvent(ctx, protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
			Breed: breedIDs[0], Success: true, Steps: make(map[string]protocol.StepResult),
		}))
	}
}

func (h *WSHandler) handleA2AHandoff(ctx context.Context, thread *a2a.Thread, fromBreed, toBreed, sessionID, artifact string) {
	if thread == nil {
		return
	}
	h.platform.A2AHub.Handoff(thread, a2a.Handoff{FromBreed: fromBreed, ToBreed: toBreed, Artifact: artifact})
	if action := h.platform.SOP.CheckA2ADepth(thread); action == sop.EscalateToCVO {
		h.sendBarkError(sessionID, toBreed, "A2A depth limit exceeded, escalated")
		return
	}
	if reviewer := sop.SelectReviewer(fromBreed, thread.Participants, sop.ReviewPolicy{RequireDifferentBreed: true}); reviewer != "" && reviewer != toBreed {
		fmt.Printf("A2A handoff: selected reviewer %s for %s → %s\n", reviewer, fromBreed, toBreed)
	}
	h.executeWithPlatform(ctx, toBreed, sessionID, artifact)
}

func detectMentionInResponse(response string) []string {
	matches := mentionRegex.FindAllStringSubmatch(response, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			result = append(result, m[1])
		}
	}
	return result
}
