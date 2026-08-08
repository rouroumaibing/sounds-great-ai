package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/hooks"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/telemetry"
	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"

	"github.com/cloudwego/eino/schema"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const maxConcurrentBark = 8

type WSHandler struct {
	upgrader    websocket.Upgrader
	mu          sync.RWMutex
	streamers   map[string]*Streamer
	pack        *pack.Pack
	platform    *platform.Platform // optional, nil = legacy mode
	sem         chan struct{}
	rateMonitor *RateMonitor
}

func NewWSHandler(p *pack.Pack) *WSHandler {
	return &WSHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			EnableCompression: true,
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
		},
		streamers:   make(map[string]*Streamer),
		pack:        p,
		sem:         make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(nil),
	}
}

// NewWSHandlerWithPlatform creates a WSHandler with platform adapter support.
// When platform is set, execution goes through CLI adapters instead of pack.Bark().
func NewWSHandlerWithPlatform(p *pack.Pack, pl *platform.Platform) *WSHandler {
	return &WSHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			EnableCompression: true,
			ReadBufferSize:    1024,
			WriteBufferSize:   1024,
		},
		streamers: make(map[string]*Streamer),
		pack:      p,
		platform:  pl,
		sem:       make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(func(sessionID string, count int) {
			log.Printf("WARN: broadcast rate exceeded for session %s: %d events/1s", sessionID, count)
		}),
	}
}

func (h *WSHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	streamer := NewStreamer(conn)
	sessionID := ""

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	stopPing := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := streamer.SendPing(); err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()
	defer close(stopPing)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var ev protocol.Event
		if err := json.Unmarshal(data, &ev); err != nil {
			errEvent := protocol.NewEvent("ERROR", "", map[string]string{
				"error": "invalid message format",
			})
			streamer.SendEvent(context.Background(), errEvent)
			continue
		}

		if ev.Type == protocol.EventUserInput {
			var payload protocol.UserInputPayload
			json.Unmarshal(ev.Payload, &payload)

			sessionID = payload.SessionID
			if sessionID == "" {
				sessionID = ev.SessionID
			}

			h.mu.Lock()
			h.streamers[sessionID] = streamer
			h.mu.Unlock()

			// Parse @mention to determine breed(s)
			var breedID string
			var routingDecision *platform.RoutingDecision
			if h.platform != nil && h.platform.MentionRouter != nil {
				rd := h.platform.MentionRouter.Route(payload.Message)
				routingDecision = &rd
				breedID = rd.TargetBreeds[0]
			} else {
				breedID = parseMention(payload.Message, h.pack)
			}

			// Push BARK_START immediately
			startEvent := protocol.NewEvent(protocol.EventBarkStart, sessionID, &protocol.BarkStartPayload{
				Breed:     breedID,
				SessionID: sessionID,
				Query:     payload.Message,
			})
			streamer.SendEvent(context.Background(), startEvent)

			// Run Bark in goroutine with session-scoped context
			go func(breedID, sessionID, query string, rd *platform.RoutingDecision) {
				// Acquire semaphore
				h.sem <- struct{}{}
				defer func() { <-h.sem }()

				// Use context that survives client disconnect
				barkCtx := context.WithoutCancel(context.Background())

				if h.platform != nil {
					if rd != nil && rd.Strategy == "serial" {
						h.executeSerial(barkCtx, rd.TargetBreeds, sessionID, query)
					} else if rd != nil && rd.Strategy == "parallel" {
						h.executeParallel(barkCtx, rd.TargetBreeds, sessionID, query)
					} else {
						h.executeWithPlatform(barkCtx, breedID, sessionID, query)
					}
					return
				}

				input := &pack.TaskInput{
					Query: query,
					Context: &pack.ExecutionContext{
						SessionID: sessionID,
					},
					Sink: streamer,
				}

				out, err := h.pack.Bark(barkCtx, breedID, input)
				if err != nil {
					errEvent := protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
						Breed: breedID,
						Error: err.Error(),
					})
					h.mu.RLock()
					s := h.streamers[sessionID]
					h.mu.RUnlock()
					if s != nil {
						s.SendEvent(context.Background(), errEvent)
					}
					return
				}

				// Build step results
				steps := make(map[string]protocol.StepResult)
				if stepData, ok := out.Data["steps"].(map[string]*pack.TaskOutput); ok {
					for stepID, stepOut := range stepData {
						steps[stepID] = protocol.StepResult{
							Approved: stepOut.Approved,
							Reason:   stepOut.Reason,
						}
					}
				}

				resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
					Breed:   breedID,
					Success: true,
					Steps:   steps,
				})
				h.mu.RLock()
				s := h.streamers[sessionID]
				h.mu.RUnlock()
				if s != nil {
					s.SendEvent(context.Background(), resultEvent)
				}
			}(breedID, sessionID, payload.Message, routingDecision)
		}

		if ev.Type == protocol.EventHitlResponse {
			var payload protocol.HitlResponsePayload
			json.Unmarshal(ev.Payload, &payload)
			log.Printf("HITL_RESPONSE received: request_id=%s approved=%v reason=%s", payload.RequestID, payload.Approved, payload.Reason)
			// TODO: Forward to agent/hitl channel when HITL flow is fully integrated
			continue
		}
	}

	if sessionID != "" {
		h.mu.Lock()
		delete(h.streamers, sessionID)
		h.mu.Unlock()
		h.rateMonitor.RemoveSession(sessionID)
	}
}

func (h *WSHandler) GetStreamer(sessionID string) *Streamer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.streamers[sessionID]
}

// SessionCount returns the number of tracked sessions in the rate monitor.
func (h *WSHandler) SessionCount() int {
	return h.rateMonitor.SessionCount()
}

// executeWithPlatform routes through the platform's CLI adapters instead of pack.Bark().
// Returns the response text (empty on error).
func (h *WSHandler) executeWithPlatform(ctx context.Context, breedID, sessionID, query string) string {
	h.mu.RLock()
	streamer := h.streamers[sessionID]
	h.mu.RUnlock()
	if streamer == nil {
		return ""
	}

	// Get breed config from platform
	breed := h.platform.GetBreed(breedID)
	if breed == nil {
		h.sendBarkError(sessionID, breedID, fmt.Sprintf("breed %q not found in platform", breedID))
		return ""
	}

	// Get default variant → CLI adapter
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

	// Build system prompt via prompt builder (identity + roster + rules + skills)
	systemPrompt := variant.SystemPrompt
	ragContext := h.retrieveRAGContext(ctx, breedID, query)
	if h.platform.PromptBuilder != nil {
		systemPrompt = h.platform.PromptBuilder.Build(prompt.BuildRequest{
			BreedID:    breedID,
			VariantID:  variant.ID,
			RAGContext: ragContext,
		})
	}

	// Inject session-init hooks (S1-S4 + S5-S8) and per-turn hooks (D1-D2)
	systemPromptL0 := ""
	if h.platform.HookPipeline != nil {
		hookInput := &hooks.AssemblerInput{
			BreedID:         breedID,
			BreedName:       breed.DisplayName,
			RoleDescription: breed.RoleDescription,
			Personality:     breed.Personality,
			CurrentPhase:    currentPhase(),
			ToolCallCount:   h.countToolCalls(sessionID),
			TaskID:          sessionID,
		}
		h.populateLeaderContext(hookInput)
		// session-init hooks
		initResult := h.platform.HookPipeline.ExecuteStage("session-init", hookInput)
		initPrompt := hooks.AssemblePatches(initResult.Patches)
		// per-turn hooks
		turnResult := h.platform.HookPipeline.ExecuteStage("per-turn", hookInput)
		turnPrompt := hooks.AssemblePatches(turnResult.Patches)
		// Native L0: Claude/Codex get session-init via CLI flag (compression-immune).
		// Other adapters: all hooks via stdin.
		if supportsNativeL0(variant.CLI.Command) {
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
		// Persist hook trace events
		if h.platform.HookTraceStore != nil {
			allEvents := append(initResult.Events, turnResult.Events...)
			if len(allEvents) > 0 {
				turnID := time.Now().Format(time.RFC3339Nano)
				if err := h.platform.HookTraceStore.Persist(sessionID, turnID, allEvents); err != nil {
					log.Printf("warning: hook trace persist failed: %v", err)
				}
			}
		}
	}

	// Load conversation history and build messages list
	var messages []*schema.Message
	if h.platform.MessageStore != nil {
		// Store user message — mark as leader if it starts with a leader mention
		sender := ""
		if h.platform.Leader != nil && isLeaderMention(query, h.platform.Leader.MentionPatterns) {
			sender = "leader"
		}
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID:  sessionID,
			Role:      "user",
			Content:   query,
			Sender:    sender,
			Timestamp: time.Now(),
		})

		// Load recent history (exclude the message we just added)
		history, _ := h.platform.MessageStore.GetByThread(sessionID, 21) // +1 to exclude current
		var contextMsgs []prompt.ContextMessage
		for _, msg := range history {
			if msg.Content == query && msg.Role == "user" {
				continue // skip current message — we'll add it as the last message
			}
			contextMsgs = append(contextMsgs, prompt.ContextMessage{
				Role:      msg.Role,
				Content:   msg.Content,
				Sender:    msg.Sender,
				Timestamp: msg.Timestamp,
			})
		}

		// Convert history to schema messages
		messages = prompt.ToSchemaMessages(contextMsgs)
	}

	// Append current query as the final user message
	messages = append(messages, schema.UserMessage(query))

	// Build execution request
	req := unified.ExecuteRequest{
		Messages:       messages,
		SystemPrompt:   systemPrompt,
		SystemPromptL0: systemPromptL0,
		Model:          variant.DefaultModel,
		WorkDir:        h.platform.WorkspaceDir,
		MCPConfig:      h.platform.BuildMCPConfig(),
	}

	// Telemetry: start span for breed execution
	execCtx := ctx
	var span trace.Span
	execStart := time.Now()
	if telemetry.IsInitialized() {
		tracer := otel.Tracer("sounds-great-ai")
		execCtx, span = tracer.Start(ctx, "breed.execute")
		span.SetAttributes(attribute.String("breed", breedID))
	}

	// Execute via CLI adapter
	eventCh, err := adapter.Execute(execCtx, req)
	if err != nil {
		if span != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
		}
		if telemetry.IsInitialized() {
			if telemetry.InvocationCompleted != nil {
				telemetry.InvocationCompleted.Add(execCtx, 1,
					metric.WithAttributes(
						attribute.String("breed", breedID),
						attribute.String("status", "error"),
					))
			}
		}
		h.sendBarkError(sessionID, breedID, fmt.Sprintf("execute failed: %v", err))
		return ""
	}

	// Stream events → WebSocket, collect response text
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

	// Telemetry: end span + record metrics + write to TraceStore
	if span != nil {
		span.End()
	}
	if telemetry.IsInitialized() {
		status := "ok"
		duration := time.Since(execStart)
		if telemetry.InvocationDuration != nil {
			telemetry.InvocationDuration.Record(execCtx, duration.Milliseconds(),
				metric.WithAttributes(attribute.String("breed", breedID)))
		}
		if telemetry.InvocationCompleted != nil {
			telemetry.InvocationCompleted.Add(execCtx, 1,
				metric.WithAttributes(
					attribute.String("breed", breedID),
					attribute.String("status", status),
				))
		}
		if ts := telemetry.TraceStoreInstance(); ts != nil {
			s := telemetry.Span{
				TraceID:   span.SpanContext().TraceID().String(),
				SpanID:    span.SpanContext().SpanID().String(),
				Name:      "breed.execute",
				StartTime: execStart,
				EndTime:   time.Now(),
				Attributes: map[string]interface{}{
					"breed":    breedID,
					"threadID": sessionID,
					"duration": duration.Milliseconds(),
				},
				Status: status,
			}
			if r := telemetry.RedactorInstance(); r != nil {
				r.RedactSpan(&s)
			}
			ts.Add(s)
		}
	}

	// Store assistant response in message store
	if h.platform.MessageStore != nil && responseText.Len() > 0 {
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID:  sessionID,
			Role:      "assistant",
			Content:   responseText.String(),
			Sender:    breedID,
			Timestamp: time.Now(),
		})
	}

	// Send BARK_RESULT
	resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
		Breed:   breedID,
		Success: true,
		Steps:   make(map[string]protocol.StepResult),
	})
	streamer.SendEvent(ctx, resultEvent)

	// Check response for @mention to trigger A2A handoff
	if h.platform != nil && h.platform.A2AHub != nil && h.platform.SOP != nil {
		mentions := detectMentionInResponse(responseText.String())
		for _, toBreed := range mentions {
			if toBreed == breedID {
				continue // don't handoff to self
			}
			if h.platform.GetBreed(toBreed) == nil {
				continue // target breed not registered
			}
			// Create or get thread for this session
			thread := h.platform.A2AHub.GetThread(sessionID)
			if thread == nil {
				thread = h.platform.A2AHub.CreateThread(query, []string{breedID})
			}
			h.handleA2AHandoff(ctx, thread, breedID, toBreed, sessionID, responseText.String())
		}
	}

	return responseText.String()
}

// executeSerial runs multiple breeds in sequence, passing each breed's output
// as context to the next breed.
func (h *WSHandler) executeSerial(ctx context.Context, breedIDs []string, sessionID, query string) {
	previousOutput := ""
	for i, breedID := range breedIDs {
		// For subsequent breeds, append previous output as context
		currentQuery := query
		if i > 0 && previousOutput != "" {
			currentQuery = fmt.Sprintf("%s\n\n[%s 的输出]:\n%s", query, breedIDs[i-1], previousOutput)
		}

		// Send BARK_START for this breed
		h.mu.RLock()
		streamer := h.streamers[sessionID]
		h.mu.RUnlock()
		if streamer != nil {
			startEvent := protocol.NewEvent(protocol.EventBarkStart, sessionID, &protocol.BarkStartPayload{
				Breed:     breedID,
				SessionID: sessionID,
				Query:     currentQuery,
			})
			streamer.SendEvent(ctx, startEvent)
		}

		previousOutput = h.executeWithPlatform(ctx, breedID, sessionID, currentQuery)
	}
}

// executeParallel runs multiple breeds concurrently, streaming interleaved events.
// Mirrors clowder-ai's route-parallel.ts — Promise.all + shared stream.
func (h *WSHandler) executeParallel(ctx context.Context, breedIDs []string, sessionID, query string) {
	// Store user message once before launching goroutines
	if h.platform.MessageStore != nil {
		sender := ""
		if h.platform.Leader != nil && isLeaderMention(query, h.platform.Leader.MentionPatterns) {
			sender = "leader"
		}
		h.platform.MessageStore.Append(&threadstore.Message{
			ThreadID:  sessionID,
			Role:      "user",
			Content:   query,
			Sender:    sender,
			Timestamp: time.Now(),
		})
	}

	// Load shared conversation history once
	var history []*threadstore.Message
	if h.platform.MessageStore != nil {
		history, _ = h.platform.MessageStore.GetByThread(sessionID, 21)
	}

	// Build shared context messages (exclude current query)
	var contextMsgs []prompt.ContextMessage
	for _, msg := range history {
		if msg.Content == query && msg.Role == "user" {
			continue
		}
		contextMsgs = append(contextMsgs, prompt.ContextMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Sender:    msg.Sender,
			Timestamp: msg.Timestamp,
		})
	}
	sharedSchemaMsgs := prompt.ToSchemaMessages(contextMsgs)
	sharedSchemaMsgs = append(sharedSchemaMsgs, schema.UserMessage(query))

	// Launch goroutines for each breed
	var wg sync.WaitGroup
	responses := make([]struct {
		breedID string
		text    string
	}, len(breedIDs))

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

			// Get breed config
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

			// Build system prompt
			systemPrompt := variant.SystemPrompt
			ragContext := h.retrieveRAGContext(ctx, bid, query)
			if h.platform.PromptBuilder != nil {
				systemPrompt = h.platform.PromptBuilder.Build(prompt.BuildRequest{
					BreedID:    bid,
					VariantID:  variant.ID,
					RAGContext: ragContext,
				})
			}

			// Inject session-init hooks (S1-S4 + S5-S8) and per-turn hooks (D1-D2)
			systemPromptL0 := ""
			if h.platform.HookPipeline != nil {
				hookInput := &hooks.AssemblerInput{
					BreedID:         bid,
					BreedName:       breed.DisplayName,
					RoleDescription: breed.RoleDescription,
					Personality:     breed.Personality,
					CurrentPhase:    currentPhase(),
					ToolCallCount:   h.countToolCalls(sessionID),
					TaskID:          sessionID,
				}
				h.populateLeaderContext(hookInput)
				// session-init hooks
				initResult := h.platform.HookPipeline.ExecuteStage("session-init", hookInput)
				initPrompt := hooks.AssemblePatches(initResult.Patches)
				// per-turn hooks
				turnResult := h.platform.HookPipeline.ExecuteStage("per-turn", hookInput)
				turnPrompt := hooks.AssemblePatches(turnResult.Patches)
				// Native L0: Claude/Codex get session-init via CLI flag (compression-immune).
				// Other adapters: all hooks via stdin.
				if supportsNativeL0(variant.CLI.Command) {
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
				// Persist hook trace events
				if h.platform.HookTraceStore != nil {
					allEvents := append(initResult.Events, turnResult.Events...)
					if len(allEvents) > 0 {
						turnID := time.Now().Format(time.RFC3339Nano)
						if err := h.platform.HookTraceStore.Persist(sessionID, turnID, allEvents); err != nil {
							log.Printf("warning: hook trace persist failed: %v", err)
						}
					}
				}
			}

			// Execute with shared messages
			req := unified.ExecuteRequest{
				Messages:       sharedSchemaMsgs,
				SystemPrompt:   systemPrompt,
				SystemPromptL0: systemPromptL0,
				Model:          variant.DefaultModel,
				WorkDir:        h.platform.WorkspaceDir,
				MCPConfig:      h.platform.BuildMCPConfig(),
			}

			eventCh, err := adapter.Execute(ctx, req)
			if err != nil {
				h.sendBarkError(sessionID, bid, fmt.Sprintf("execute failed: %v", err))
				return
			}

			// Stream events + collect response
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

	// Store all breed responses
	if h.platform.MessageStore != nil {
		for _, resp := range responses {
			if resp.text != "" {
				h.platform.MessageStore.Append(&threadstore.Message{
					ThreadID:  sessionID,
					Role:      "assistant",
					Content:   resp.text,
					Sender:    resp.breedID,
					Timestamp: time.Now(),
				})
			}
		}
	}

	// Send BARK_RESULT
	h.mu.RLock()
	streamer := h.streamers[sessionID]
	h.mu.RUnlock()
	if streamer != nil {
		resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
			Breed:   breedIDs[0],
			Success: true,
			Steps:   make(map[string]protocol.StepResult),
		})
		streamer.SendEvent(ctx, resultEvent)
	}
}

// detectMentionInResponse extracts @breedID tokens from a breed's response text.
// Returns deduplicated breed IDs in order of first appearance.
// Uses mentionRegex defined in mention.go.
func detectMentionInResponse(response string) []string {
	matches := mentionRegex.FindAllStringSubmatch(response, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		breedID := m[1]
		if !seen[breedID] {
			seen[breedID] = true
			result = append(result, breedID)
		}
	}
	return result
}

// handleA2AHandoff processes an A2A task handoff from one breed to another.
// It records the handoff in the thread, checks SOP A2A depth gate,
// selects a reviewer, and executes the target breed.
func (h *WSHandler) handleA2AHandoff(ctx context.Context, thread *a2a.Thread, fromBreed, toBreed, sessionID, artifact string) {
	if thread == nil {
		return
	}

	// 1. Record handoff in thread
	hf := a2a.Handoff{FromBreed: fromBreed, ToBreed: toBreed, Artifact: artifact}
	h.platform.A2AHub.Handoff(thread, hf)

	// 2. SOP gate: check A2A depth
	action := h.platform.SOP.CheckA2ADepth(thread)
	if action == sop.EscalateToCVO {
		h.sendBarkError(sessionID, toBreed, "A2A depth limit exceeded, escalated")
		return
	}

	// 3. Select reviewer for cross-breed review
	reviewer := sop.SelectReviewer(fromBreed, thread.Participants, sop.ReviewPolicy{
		RequireDifferentBreed: true,
	})
	if reviewer != "" && reviewer != toBreed {
		log.Printf("A2A handoff: selected reviewer %s for %s → %s", reviewer, fromBreed, toBreed)
	}

	// 4. Execute target breed with handoff artifact as query
	h.executeWithPlatform(ctx, toBreed, sessionID, artifact)
}

func (h *WSHandler) sendBarkError(sessionID, breedID, errMsg string) {
	h.mu.RLock()
	s := h.streamers[sessionID]
	h.mu.RUnlock()
	if s != nil {
		errEvent := protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
			Breed: breedID,
			Error: errMsg,
		})
		s.SendEvent(context.Background(), errEvent)
	}
}

// SendSystemNotice 向所有连接的 WebSocket 客户端广播系统通知。
// 用于 BurnRateMonitor 推送告警/恢复消息。
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

// countToolCalls estimates the tool call count for a session by counting
// assistant messages in the thread. Each assistant turn typically involves
// one or more tool calls.
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

// populateLeaderContext fills Leader fields in AssemblerInput from the
// platform's LeaderConfig. No-op if platform or Leader is nil.
func (h *WSHandler) populateLeaderContext(input *hooks.AssemblerInput) {
	if h.platform == nil || h.platform.Leader == nil {
		return
	}
	leader := h.platform.Leader
	input.LeaderName = leader.Name
	input.LeaderFirstMention = leader.MentionPatterns[0]
	// Format handles from mention patterns
	parts := make([]string, len(leader.MentionPatterns))
	for i, p := range leader.MentionPatterns {
		parts[i] = "`" + p + "`"
	}
	input.LeaderHandles = strings.Join(parts, " / ")
}

// currentPhase returns the current Phase from env var CURRENT_PHASE,
// defaulting to "7" if not set.
func currentPhase() string {
	phase := os.Getenv("CURRENT_PHASE")
	if phase == "" {
		return "7"
	}
	return phase
}

// supportsNativeL0 returns true if the CLI adapter supports native L0
// system prompt injection (compression-immune via CLI flag).
func supportsNativeL0(cliCommand string) bool {
	return cliCommand == "claude" || cliCommand == "codex"
}

// convertStreamEvent maps a unified.StreamEvent to a protocol.Event.
func convertStreamEvent(ev unified.StreamEvent, sessionID, breedID string) *protocol.Event {
	switch ev.Type {
	case "thinking":
		return protocol.NewEvent(protocol.EventThinking, sessionID, &protocol.ThinkingPayload{
			Step:    1,
			Content: ev.Content,
		})
	case "tool_call":
		toolName, _ := ev.Meta["tool"].(string)
		return protocol.NewEvent(protocol.EventToolCall, sessionID, &protocol.ToolCallPayload{
			Tool:   toolName,
			Params: ev.Content,
		})
	case "error":
		return protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
			Breed: breedID,
			Error: ev.Content,
		})
	default:
		return nil
	}
}

// ragEnabled returns true if RAG is enabled via RAG_ENABLED env var.
// Defaults to true (opt-out via RAG_ENABLED=false).
func ragEnabled() bool {
	v := os.Getenv("RAG_ENABLED")
	if v == "" {
		return true
	}
	return v == "true" || v == "1" || v == "yes"
}

// breedHasRetrieverRole checks if the breed config has the "retriever" role.
func breedHasRetrieverRole(breed *config.BreedConfig) bool {
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

// retrieveRAGContext executes retrieve → context_assemble for the given query
// and returns the assembled RAG context string. Returns empty string if RAG
// is disabled, the breed is not a retriever, or retrieval yields no results.
// All errors are handled gracefully (logged + empty return) — RAG is best-effort.
func (h *WSHandler) retrieveRAGContext(ctx context.Context, breedID, query string) string {
	if !ragEnabled() {
		return ""
	}
	if h.platform == nil || h.platform.RAGRegistry == nil {
		return ""
	}
	breed := h.platform.GetBreed(breedID)
	if !breedHasRetrieverRole(breed) {
		return ""
	}

	// 1. Retrieve
	retrieveCap := capability.NewRetrieveCapability(h.platform.RAGRegistry, h.platform.Embedder)
	retrieveOut, err := retrieveCap.Run(ctx, &pack.TaskInput{Query: query})
	if err != nil || retrieveOut == nil {
		return ""
	}
	matches, _ := retrieveOut.Data["matches"].([]any)
	if len(matches) == 0 {
		return ""
	}

	// 2. Context assemble
	assembleCap := capability.NewContextAssemble()
	assembleOut, err := assembleCap.Run(ctx, &pack.TaskInput{
		Previous: map[string]*pack.TaskOutput{
			"retrieve": retrieveOut,
		},
	})
	if err != nil || assembleOut == nil {
		return ""
	}
	context, _ := assembleOut.Data["context"].(string)
	return context
}
