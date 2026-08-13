package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/prompt"
	"sounds-great-ai/internal/telemetry"
	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	agentsPorts "sounds-great-ai/internal/domains/agents/ports"
	sopPorts "sounds-great-ai/internal/domains/sop/ports"
	routingPorts "sounds-great-ai/internal/domains/routing/ports"
	"sounds-great-ai/pkg/protocol"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ballHeartbeatInterval is the cadence at which a running invocation emits a
// ball-custody heartbeat. The reconciler treats a missing heartbeat as a hung
// invocation (see platform.StartReconciler).
const ballHeartbeatInterval = 30 * time.Second

// worklistMaxDepth floors the per-invocation A2A depth ceiling at the ping-pong
// block threshold (G2) so the streak breaker is the active loop-stopper even
// when SOP's per-thread depth default (3) is lower.
func worklistMaxDepth(sopMax int) int {
	if sopMax >= routingPorts.PingPongBlockThreshold {
		return sopMax
	}
	return 8
}

// recordBall writes a ball-custody ledger event without affecting orchestration
// control flow. The ledger is a passive observer during P0 ("只写不读"): any error
// is swallowed so a ledger failure can never break a dog run.
func (h *WSHandler) recordBall(ctx context.Context, fn func(l custodyPorts.IBallLedger) error) {
	if h.platform == nil || h.platform.BallLedger == nil {
		return
	}
	_ = fn(h.platform.BallLedger)
}

// tryDispatch performs a guarded dispatch disposition (G1): it converges the
// ball from `from` to `to` only if `from` is still the current holder. Returns
// false when the guard rejects the disposition (stale/duplicate callback), so
// the caller can skip executing the handoff instead of wrongly resolving the ball.
func (h *WSHandler) tryDispatch(ctx context.Context, sessionID, from, to string) bool {
	if h.platform == nil || h.platform.BallLedger == nil {
		return false
	}
	ok, err := h.platform.BallLedger.TryDispatchDispositioned(ctx, sessionID, from, to)
	if err != nil {
		return false
	}
	return ok
}

func (h *WSHandler) executeWithPlatform(ctx context.Context, breedID, sessionID, query, invID string, suppressHandoff bool) string {
	h.mu.RLock()
	streamer := h.streamers[sessionID]
	h.mu.RUnlock()
	if streamer == nil {
		return ""
	}

	// G2: register this invocation's worklist (idempotent) so A2A handoffs
	// share one depth/streak budget for the whole user message. The depth
	// ceiling is kept at/above the ping-pong block threshold (4) so the streak
	// breaker (not the depth guard) is the primary loop-stopper; SOP's
	// cumulative per-thread depth limit remains a separate backstop.
	if invID != "" && h.platform != nil && h.platform.Worklist != nil {
		h.platform.Worklist.Register(invID, worklistMaxDepth(h.platform.SOP.MaxA2ADepth()))
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
		h.platform.MessageStore.Append(&threadPorts.Message{
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
	// G7 step 3: never let burst-window truncation orphan the most recent Q→A
	// pair(s) — keep the last 4 turns intact.
	messages = prompt.ProtectRecentPairs(messages, 4)
	messages = append(messages, schema.UserMessage(query))

	req := agentsPorts.ExecuteRequest{
		ClientID:      variant.ClientID,
		Messages:      messages,
		SystemPrompt:  systemPrompt,
		SystemPromptL0: systemPromptL0,
		Model:         variant.DefaultModel,
		WorkDir:       h.platform.WorkspaceDir,
		MCPConfig:     h.platform.BuildMCPConfig(),
		ThreadID:      sessionID,
		SessionID:     sessionID,
		Context:       ctx,
	}

	execCtx := ctx
	var span trace.Span
	execStart := time.Now()
	if telemetry.IsInitialized() {
		tracer := otel.Tracer("sounds-great-ai")
		execCtx, span = tracer.Start(ctx, "breed.execute")
		span.SetAttributes(attribute.String("breed", breedID))
	}

	// Ball custody ledger (P0 shadow write; never affects control flow).
	h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
		return l.RecordHanded(ctx, sessionID, "", breedID)
	})
	h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
		return l.RecordInvocationStarted(ctx, sessionID, breedID)
	})

	eventCh, err := h.platform.AgentExecutor.Execute(execCtx, req)
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
		h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
			return l.RecordInvocationDied(ctx, sessionID, breedID)
		})
		return ""
	}

	// Heartbeat: keep the ball-custody ledger alive while this invocation runs.
	// The reconciler sweep (Platform.StartReconciler) marks invocations that stop
	// heartbeating as died→zombie, mirroring clowder-ai's invocation.heartbeat.
	heartbeatStop := make(chan struct{})
	go func() {
		hb := time.NewTicker(ballHeartbeatInterval)
		defer hb.Stop()
		for {
			select {
			case <-heartbeatStop:
				return
			case <-hb.C:
				h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
					return l.RecordInvocationHeartbeat(ctx, sessionID, breedID)
				})
			}
		}
	}()
	defer close(heartbeatStop)

	var responseText strings.Builder
	// Streaming filter strips the hold_ball control fence (```hold_ball ... ```)
	// from the live output so the user never sees the raw marker. The condition
	// is re-parsed from the full accumulated text after the stream ends.
	filter := newHoldMarkerFilter(func(s string) {
		if s == "" {
			return
		}
		protoEvent := convertStreamEvent(agentsPorts.StreamEvent{Type: "text", Content: s}, sessionID, breedID)
		if protoEvent != nil {
			h.rateMonitor.Record(sessionID)
			streamer.SendEvent(ctx, protoEvent)
		}
	})
	for ev := range eventCh {
		if ev.Type == "text" {
			responseText.WriteString(ev.Content)
			filter.push(ev.Content)
			continue
		}
		// Non-text event: flush any pending stripped text, then forward as-is.
		filter.flush()
		protoEvent := convertStreamEvent(ev, sessionID, breedID)
		if protoEvent != nil {
			h.rateMonitor.Record(sessionID)
			streamer.SendEvent(ctx, protoEvent)
		}
	}
	filter.flush()

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

	cleaned := responseText.String()
	cond, cleaned, held := extractHoldCondition(cleaned)

	if held {
		// Park the ball: write ball.held, register the hold, and stop here.
		// The thread stays parked; no task.done and no A2A recursion until the
		// hold is released (manual click / webhook) and the holder is resumed.
		h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
			return l.RecordHeld(ctx, sessionID, breedID)
		})
		if h.platform != nil && h.platform.HoldScheduler != nil {
			_ = h.platform.HoldBall(ctx, sessionID, breedID, cond, query)
		}
		if h.platform.MessageStore != nil && cleaned != "" {
			h.platform.MessageStore.Append(&threadPorts.Message{
				ThreadID: sessionID, Role: "assistant", Content: cleaned, Sender: breedID, Timestamp: time.Now(),
			})
		}
		streamer.SendEvent(ctx, protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
			Breed: breedID, Success: true, Steps: make(map[string]protocol.StepResult),
		}))
		h.SendSystemNotice("info", "线程已挂起", "狗狗已持球等待（hold_ball），满足条件后将自动继续。")
		return cleaned
	}

	if h.platform.MessageStore != nil && cleaned != "" {
		h.platform.MessageStore.Append(&threadPorts.Message{
			ThreadID: sessionID, Role: "assistant", Content: cleaned, Sender: breedID, Timestamp: time.Now(),
		})
	}

	h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
		return l.RecordTaskDone(ctx, sessionID, breedID)
	})

	resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
		Breed: breedID, Success: true, Steps: make(map[string]protocol.StepResult),
	})
	streamer.SendEvent(ctx, resultEvent)

	if !suppressHandoff && h.platform != nil && h.platform.A2AHub != nil && h.platform.SOP != nil {
		mentions := detectMentionInResponse(cleaned)
		for _, toBreed := range mentions {
			if toBreed == breedID {
				continue
			}
			if h.platform.GetBreed(toBreed) == nil {
				// G4: a handoff with no valid target is a voided pass.
				h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
					return l.Record(ctx, custodyPorts.BallEvent{
						ThreadID: sessionID, Type: custodyPorts.BallVoidPass, From: breedID, To: toBreed,
					})
				})
				continue
			}
			thread := h.platform.A2AHub.GetThread(ctx, sessionID)
			if thread == nil {
				thread = h.platform.A2AHub.CreateThread(ctx, query, []string{breedID})
			}
			h.handleA2AHandoff(ctx, thread, breedID, toBreed, sessionID, invID, cleaned)
		}
	}

	return cleaned
}

// holdFenceOpen / holdFenceClose delimit the dog-declared hold_ball control
// signal. A dog emits a fenced block at the end of its turn to park the ball:
//
//	```hold_ball
//	{"kind":"manual"}
//	```
//
// Only "manual" and "webhook" kinds are honored in this phase (D3); the fence
// is stripped from the streamed output so the user never sees the raw JSON.
const (
	holdFenceOpen  = "```hold_ball"
	holdFenceClose = "```"
)

// extractHoldCondition parses a ```hold_ball ... ``` fence from the dog's
// response. It returns the wake condition, the response with the fence removed,
// and whether a hold was requested. A bare ```hold_ball (no JSON body) defaults
// to a manual wake.
func extractHoldCondition(text string) (custodyPorts.WakeCondition, string, bool) {
	open := strings.Index(text, holdFenceOpen)
	if open < 0 {
		return custodyPorts.WakeCondition{}, text, false
	}
	rest := text[open+len(holdFenceOpen):]
	close := strings.Index(rest, holdFenceClose)
	if close < 0 {
		// Unterminated fence — treat as no hold rather than corrupting output.
		return custodyPorts.WakeCondition{}, text, false
	}
	jsonBody := strings.TrimSpace(rest[:close])
	cleaned := text[:open] + strings.TrimSpace(rest[close+len(holdFenceClose):])
	cond := custodyPorts.WakeCondition{Kind: custodyPorts.WakeManual}
	if jsonBody != "" {
		_ = json.Unmarshal([]byte(jsonBody), &cond)
	}
	if cond.Kind == "" {
		cond.Kind = custodyPorts.WakeManual
	}
	return cond, cleaned, true
}

// holdMarkerFilter removes ```hold_ball fences from a stream of text chunks so
// the control signal never reaches the user. It tolerates the fence being split
// across chunk boundaries.
type holdMarkerFilter struct {
	out     func(string)
	buf     strings.Builder
	inFence bool
}

func newHoldMarkerFilter(out func(string)) *holdMarkerFilter {
	return &holdMarkerFilter{out: out}
}

func (f *holdMarkerFilter) push(chunk string) {
	f.buf.WriteString(chunk)
	data := f.buf.String()
	for {
		if !f.inFence {
			open := strings.Index(data, holdFenceOpen)
			if open < 0 {
				// No open fence. Emit everything unless the tail is a prefix of
				// the open marker (a partial fence split across chunks).
				if tailStart := strings.LastIndex(data, "```"); tailStart >= 0 {
					tail := data[tailStart:]
					if strings.HasPrefix(holdFenceOpen, tail) {
						f.out(data[:tailStart])
						f.buf.Reset()
						f.buf.WriteString(tail)
						return
					}
				}
				f.out(data)
				f.buf.Reset()
				return
			}
			if open > 0 {
				f.out(data[:open])
			}
			data = data[open:]
			f.inFence = true
		}
		rest := data[len(holdFenceOpen):]
		close := strings.Index(rest, holdFenceClose)
		if close < 0 {
			f.buf.Reset()
			f.buf.WriteString(data)
			return
		}
		data = rest[close+len(holdFenceClose):]
		f.inFence = false
	}
}

func (f *holdMarkerFilter) flush() {
	if !f.inFence && f.buf.Len() > 0 {
		f.out(f.buf.String())
		f.buf.Reset()
	}
	// If still inside a fence at EOF, drop the dangling open-marker content.
}

// resumeHeld re-dispatches the holder breed after its hold has been released.
// It injects a system wake notice plus the original request so the dog recovers
// full context, then runs the normal execution path (which may hold again,
// finish, or hand off to the next breed via @mention).
func (h *WSHandler) resumeHeld(ctx context.Context, sessionID, holder, resumeMsg string) {
	wakeNotice := "[系统] 唤醒条件已满足，请继续处理之前挂起的任务。"
	query := wakeNotice
	if resumeMsg != "" {
		query += "\n\n" + resumeMsg
	}
	// A resumed hold is a fresh top-level invocation: mint a new invID so its
	// A2A chain gets its own depth/streak budget (G2).
	invID := uuid.NewString()
	h.executeWithPlatform(ctx, holder, sessionID, query, invID, false)
}

// ResumeHeldThread releases a parked hold (validating the wake kind/token) and
// re-dispatches the holder. Safe to call from the WS loop (WAKE_HOLD event) or
// an HTTP webhook endpoint. Returns an error if there is no active hold or the
// credentials do not match.
func (h *WSHandler) ResumeHeldThread(ctx context.Context, sessionID string, kind custodyPorts.WakeKind, token string) error {
	if h.platform == nil || h.platform.HoldScheduler == nil {
		return fmt.Errorf("hold scheduler unavailable")
	}
	rec, err := h.platform.WakeHold(ctx, sessionID, kind, token)
	if err != nil {
		return err
	}
	go h.resumeHeld(context.Background(), sessionID, rec.Holder, rec.ResumeMessage)
	return nil
}

func (h *WSHandler) executeSerial(ctx context.Context, breedIDs []string, sessionID, query, invID string) {
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
		// Disposition closure for the serial worklist: the previous breed
		// disposes the ball to this one (mirrors handleA2AHandoff). Each link in
		// the pipeline is recorded as dispatch_dispositioned so the custody
		// trail shows the chain rather than N independent single runs. The
		// dispatch is guarded (G1): a stale/duplicate disposition is rejected and
		// the pipeline stops rather than resolving a superseded ball.
		if i > 0 {
			if !h.tryDispatch(ctx, sessionID, breedIDs[i-1], breedID) {
				h.SendSystemNotice("warn", "传球被拒",
					"串行链中 "+breedIDs[i-1]+" → "+breedID+" 的传球已被更新的状态顶替，链路终止。")
				break
			}
		}
		previousOutput = h.executeWithPlatform(ctx, breedID, sessionID, currentQuery, invID, true)
	}
}

func (h *WSHandler) executeParallel(ctx context.Context, breedIDs []string, sessionID, query, invID string) {
	if h.platform.MessageStore != nil {
		sender := ""
		if h.platform.Leader != nil && isLeaderMention(query, h.platform.Leader.MentionPatterns) {
			sender = "leader"
		}
		h.platform.MessageStore.Append(&threadPorts.Message{
			ThreadID: sessionID, Role: "user", Content: query, Sender: sender, Timestamp: time.Now(),
		})
	}
	var history []*threadPorts.Message
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
	// G7 step 3: protect the most recent Q→A pairs from truncation.
	sharedSchemaMsgs = prompt.ProtectRecentPairs(sharedSchemaMsgs, 4)
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
			systemPrompt := variant.SystemPrompt
			ragContext := h.retrieveRAGContext(ctx, bid, query)
			if h.platform.PromptBuilder != nil {
				systemPrompt = h.platform.PromptBuilder.Build(prompt.BuildRequest{
					BreedID: bid, VariantID: variant.ID, RAGContext: ragContext,
				})
			}
			systemPrompt, systemPromptL0 := h.injectHooks(systemPrompt, bid, breed.DisplayName, breed.RoleDescription, breed.Personality, sessionID)
			req := agentsPorts.ExecuteRequest{
				ClientID:       variant.ClientID,
				Messages:       sharedSchemaMsgs,
				SystemPrompt:   systemPrompt,
				SystemPromptL0: systemPromptL0,
				Model:          variant.DefaultModel,
				WorkDir:        h.platform.WorkspaceDir,
				MCPConfig:      h.platform.BuildMCPConfig(),
				ThreadID:       sessionID,
				SessionID:      sessionID,
				Context:        ctx,
			}
			eventCh, err := h.platform.AgentExecutor.Execute(ctx, req)
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
				h.platform.MessageStore.Append(&threadPorts.Message{
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

func (h *WSHandler) handleA2AHandoff(ctx context.Context, thread *a2a.Thread, fromBreed, toBreed, sessionID, invID, artifact string) {
	if thread == nil {
		return
	}
	// Guarded disposition (G1): only converge the ball if fromBreed is still the
	// current holder. A stale or superseded handoff callback is rejected and the
	// handoff is skipped instead of wrongly resolving the ball.
	if !h.tryDispatch(ctx, sessionID, fromBreed, toBreed) {
		h.SendSystemNotice("warn", "传球被拒",
			"从 "+fromBreed+" 到 "+toBreed+" 的传球已被更新的状态顶替，跳过本次 handoff。")
		return
	}
	// G7 step 1: tag the next dog with the handoff source so it knows who called
	// it and why (context-transport.a2aFrom / a2aTriggerMessageId parity).
	enriched := artifact
	if src := buildHandoffSourceNotice(fromBreed); src != "" {
		enriched = src + "\n\n" + artifact
	}
	// G7 step 2: scrub sensitive payloads from the artifact before it crosses the
	// breed boundary (reuses telemetry.RedactorInstance + secret-pattern redaction).
	enriched = scrubHandoffContext(enriched)
	// G2: consult the per-invocation worklist before recursing. Reject the
	// handoff on depth overflow or ping-pong break (record task.blocked, stop
	// the chain); inject a warning when the streak is building.
	if invID != "" && h.platform != nil && h.platform.Worklist != nil {
		accepted, reason, warn := h.platform.Worklist.Push(invID, fromBreed, toBreed, routingPorts.SubstantiveActivity{
			OutputLen: len([]rune(artifact)),
		})
		if !accepted {
			h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
				return l.RecordTaskBlocked(ctx, sessionID, toBreed)
			})
			msg := "已达调用深度上限，停止交接。"
			if reason == "pingpong" {
				msg = "检测到你与对方反复互相 @ 调用(ping-pong)，已自动熔断以避免死循环。"
			}
			h.SendSystemNotice("warn", "交接终止", msg)
			return
		}
		if warn {
			enriched = "[系统] 警告：你与对方已连续多次互相 @ 调用，请尝试推进任务或换一种协作方式，否则将在 2 轮后自动熔断。\n\n" + enriched
		}
	}
	h.platform.A2AHub.Handoff(ctx, thread, a2a.Handoff{FromBreed: fromBreed, ToBreed: toBreed, Artifact: enriched})
	if action := h.platform.SOP.CheckA2ADepth(thread); action == sopPorts.EscalateToCVO {
		// G4: escalate to operator/CVO by parking the ball (handed_cvo → parked)
		// instead of just surfacing an error and leaving the ball unresolved.
		h.recordBall(ctx, func(l custodyPorts.IBallLedger) error {
			return l.Record(ctx, custodyPorts.BallEvent{
				ThreadID: sessionID, Type: custodyPorts.BallHandedCVO, Holder: toBreed, To: toBreed,
			})
		})
		h.SendSystemNotice("warn", "已上报 CVO", "A2A 深度超限，已将球权上交运营/主管处理。")
		return
	}
	if reviewer := h.platform.SOP.SelectReviewer(fromBreed, thread.Participants, sopPorts.ReviewPolicy{RequireDifferentBreed: true}); reviewer != "" && reviewer != toBreed {
		fmt.Printf("A2A handoff: selected reviewer %s for %s → %s\n", reviewer, fromBreed, toBreed)
	}
	h.executeWithPlatform(ctx, toBreed, sessionID, enriched, invID, false)
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
