package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/internal/memory"
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

// summarizeForContinuity turns a user query into a short, persisted note of
// "what the breed was working on" for the continuity store (Persistent Identity
// P3). It keeps only the leading content so the digest stays compact.
func summarizeForContinuity(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "(空查询)"
	}
	// Collapse internal newlines for a single-line summary.
	q = strings.Join(strings.Fields(q), " ")
	const maxLen = 200
	runes := []rune(q)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "…"
	}
	return q
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

// fireProfileDistillationTrigger emits the KD-10 eval counter when a session
// run completes — the homologous "session seal" for SG's one-shot
// model (ProfileDistillationTrigger.onSessionSealed). Beyond the observability
// counter it ALSO performs a best-effort autonomous distill: the accumulated
// evidence for the session's dog relationship key is aggregated into a pending
// capsule proposal (never auto-applied). Reasoning stays in the CLI agent /
// operator — the platform only aggregates what it already holds (docs/decisions/irreversible-decisions.md §4.1).
// Both steps are fail-closed and non-blocking: a distill failure can never
// break a dog run.
func (h *WSHandler) fireProfileDistillationTrigger(sessionID, breedID, reason string) {
	if telemetry.IsInitialized() && telemetry.ProfileDistillationTriggered != nil {
		telemetry.ProfileDistillationTriggered.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("agent.id", breedID),
			attribute.String("seal.reason", reason),
		))
	}
	// Real distill on seal: fire-and-forget so the session result is never
	// delayed by the (optional) proposal write.
	h.maybeAutoDistill(sessionID, breedID)
}

// maybeAutoDistill triggers an on-session-seal autonomous distill when the
// capsule handler is wired. It is intentionally fire-and-forget: any error or
// missing wiring degrades to a no-op (the session result is unaffected).
func (h *WSHandler) maybeAutoDistill(sessionID, breedID string) {
	if h.profiles == nil {
		return
	}
	go h.profiles.AutoDistillSession(context.Background(), sessionID, breedID)
}

// maybeSupplyLanes triggers typed-lane candidate extraction at session close.
// It is fire-and-forget and fail-closed (mirrors maybeAutoDistill): a supply
// failure can never break a dog run. The DeltaProducer is deterministic pattern
// matching — no LLM is called (VISION §3 compliance). Detected candidates land
// as pending lane entries (persisted via NewLaneRegistryAt) awaiting human
// disposition (P3).
func (h *WSHandler) maybeSupplyLanes(sessionID, breedID string) {
	if h.platform == nil || h.platform.SharedMemory == nil || h.platform.LaneSupply == nil {
		return
	}
	if h.platform.MessageStore == nil {
		return
	}
	history, err := h.platform.MessageStore.GetByThread(sessionID, 50)
	if err != nil {
		return
	}
	if len(history) == 0 {
		return
	}
	msgs := make([]memory.SessionMessage, 0, len(history))
	for _, m := range history {
		msgs = append(msgs, memory.SessionMessage{
			Role:    m.Role,
			Content: m.Content,
			Time:    m.Timestamp.UnixMilli(),
		})
	}
	reg := h.platform.SharedMemory
	supply := h.platform.LaneSupply
	operator := "operator"
	if h.platform.Leader != nil && h.platform.Leader.Name != "" {
		operator = h.platform.Leader.Name
	}
	go func() {
		// Fail-closed: a supply panic must never propagate into the session path.
		defer func() { _ = recover() }()
		delta := supply.Detect(sessionID, msgs)
		candidates := supply.Produce(delta)
		// Count the producer's intended submissions per lane for the
		// lane_candidate_submitted eval counter (homologous clowder
		// CrossCatMetricsComputer). Dedup on submit may drop some, but this
		// reflects supply pressure per lane.
		byLane := map[memory.LaneType]int64{}
		for _, c := range candidates {
			byLane[c.Lane]++
		}
		supply.SubmitCandidates(reg, candidates, operator)
		if telemetry.IsInitialized() {
			for lane, n := range byLane {
				if n > 0 && telemetry.LaneCandidateSubmitted != nil {
					telemetry.LaneCandidateSubmitted.Add(context.Background(), n, metric.WithAttributes(attribute.String("lane", string(lane))))
				}
			}
		}
	}()
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
	// Emit the truth-injected counter and record a recall event when approved
	// shared memory was recalled into this breed's system prompt (homologous
	// clowder CrossCatMetricsComputer cross-cat consumption spread + recall_events
	// observability). Only entries visible to this operator are recalled.
	if h.platform.SharedMemory != nil {
		operator := "operator"
		if h.platform.Leader != nil && h.platform.Leader.Name != "" {
			operator = h.platform.Leader.Name
		}
		if entries, ok := h.platform.SharedMemory.RecallEntries(20, operator); ok && len(entries) > 0 {
			ids := make([]string, 0, len(entries))
			for _, e := range entries {
				ids = append(ids, e.ID)
			}
			if h.platform.LaneRecall != nil {
				h.platform.LaneRecall.Record(&memory.RecallEvent{
					OperatorID: operator,
					Kind:       "push",
					Trigger:    "session_bootstrap",
					EntryIDs:   ids,
					Count:      len(ids),
				})
			}
			if telemetry.IsInitialized() && telemetry.LaneTruthInjected != nil {
				telemetry.LaneTruthInjected.Add(context.Background(), 1, metric.WithAttributes(attribute.String("agent.id", breedID)))
			}
		}
	}
	systemPrompt, systemPromptL0 := h.injectHooks(systemPrompt, breedID, breed.DisplayName, breed.RoleDescription, breed.Personality, sessionID)

	// Persistent Identity F276 (homologous recall injection): when the
	// user's message references a known third-party person, inject a token-bounded
	// relationship card into the dog's context so it "remembers" them (anchor-first
	// context entry, F236). The block is budgeted in settings so it never
	// bloats the prompt.
	if h.platform != nil && h.platform.PeopleMemory != nil {
		pmOp := "operator"
		if h.platform.Leader != nil && h.platform.Leader.Name != "" {
			pmOp = h.platform.Leader.Name
		}
		if block, rerr := h.platform.PeopleMemory.RecallContextForQuery(pmOp, query); rerr == nil && block != "" {
			systemPrompt = systemPrompt + "\n" + block
		}
	}

	// Persistent Identity P2 (homologous auto-compact budget): the
	// breed's configured compaction threshold bounds the history the platform
	// feeds the CLI. In SG's one-shot model the platform controls context
	// (not the CLI's in-session compaction), so this is the real consumer of
	// auto_compact_token_limit. Fall back to ContextBudget.MaxContextTokens.
	compactBudget := variant.AutoCompactTokenLimit
	if compactBudget <= 0 {
		compactBudget = variant.ContextBudget.MaxContextTokens
	}

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
		// Enforce the breed's auto-compact budget on the assembled history
		// (oldest messages dropped first) before it reaches the CLI.
		if compactBudget > 0 {
			contextMsgs = prompt.BoundContextByTokens(contextMsgs, compactBudget)
		}
		messages = prompt.ToSchemaMessages(contextMsgs)
	}
	// G7 step 3: never let burst-window truncation orphan the most recent Q→A
	// pair(s) — keep the last 4 turns intact.
	messages = prompt.ProtectRecentPairs(messages, 4)
	messages = append(messages, schema.UserMessage(query))

	// Persistent Identity P3 (homologous F211 continuity bootstrap):
	// record this spawn as a NEW rotation so the NEXT spawn injects a
	// "续接上下文" section (what it was working on). We advance the rotation
	// index per spawn (RecordNextRotation) instead of hardcoding 0: in one-shot
	// mode each task is its own rotation, and once a long (warm) session exists
	// the ring already holds per-rotation checkpoints (see continuity.go).
	// Best-effort — a failure to persist must never block execution.
	if h.platform.Continuity != nil {
		if _, err := h.platform.Continuity.RecordNextRotation(breedID, summarizeForContinuity(query), sessionID); err != nil {
			log.Printf("WARN: continuity record failed for breed %s: %v", breedID, err)
		}
	}

	// Persistent Identity (homologous distill): note which breed runs
	// this session so the autonomous-distill endpoint can derive the distiller
	// from the CURRENT session instead of a hardcoded default dog. Best-effort.
	if h.platform != nil {
		h.platform.RecordSessionBreed(sessionID, breedID)
	}

	req := agentsPorts.ExecuteRequest{
		ClientID:             variant.ClientID,
		Messages:             messages,
		SystemPrompt:         systemPrompt,
		SystemPromptL0:       systemPromptL0,
		Model:                variant.DefaultModel,
		WorkDir:              h.platform.WorkspaceDir,
		MCPConfig:            h.platform.BuildMCPConfig(),
		ThreadID:             sessionID,
		SessionID:            sessionID,
		Context:              ctx,
		AutoCompactTokenLimit: compactBudget,
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
	// heartbeating as died→zombie, mirroring invocation.heartbeat.
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
		// Session seal (held): fire the distillation trigger counter.
		h.fireProfileDistillationTrigger(sessionID, breedID, "held")
		// Session seal (held): extract typed-lane candidates (P2).
		h.maybeSupplyLanes(sessionID, breedID)
		if h.platform != nil && h.platform.HoldScheduler != nil {
			_ = h.platform.HoldBall(ctx, sessionID, breedID, cond, query)
		}
		if h.platform.MessageStore != nil && cleaned != "" {
			h.platform.MessageStore.Append(&threadPorts.Message{
				ThreadID: sessionID, Role: "assistant", Content: cleaned, Sender: breedID, Timestamp: time.Now(),
			})
		}
		streamer.SendEvent(ctx, protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
			Breed:   breedID,
			Success: true,
			Steps:   make(map[string]protocol.StepResult),
			Content: cleaned, // G9
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
	// Session seal (task done): fire the distillation trigger counter.
	h.fireProfileDistillationTrigger(sessionID, breedID, "task_done")
	// Session seal (task done): extract typed-lane candidates (P2).
	h.maybeSupplyLanes(sessionID, breedID)

	resultEvent := protocol.NewEvent(protocol.EventBarkResult, sessionID, &protocol.BarkResultPayload{
		Breed:   breedID,
		Success: true,
		Steps:   make(map[string]protocol.StepResult),
		Content: cleaned, // G9: carry final text so the terminal render needs no REST hydration
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
			h.handleA2AHandoff(ctx, thread, breedID, variant.ID, toBreed, sessionID, invID, cleaned)
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
			// Persistent Identity P2: carry the breed's auto-compact budget on
			// the request (the shared broadcast history is bounded once on the
			// main spawn path; here the contract field is populated per-breed).
			compactBudget := variant.AutoCompactTokenLimit
			if compactBudget <= 0 {
				compactBudget = variant.ContextBudget.MaxContextTokens
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
				ClientID:             variant.ClientID,
				Messages:             sharedSchemaMsgs,
				SystemPrompt:         systemPrompt,
				SystemPromptL0:       systemPromptL0,
				Model:                variant.DefaultModel,
				WorkDir:              h.platform.WorkspaceDir,
				MCPConfig:            h.platform.BuildMCPConfig(),
				ThreadID:             sessionID,
				SessionID:            sessionID,
				Context:              ctx,
				AutoCompactTokenLimit: compactBudget,
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

	var respParts []string
	for _, resp := range responses {
		if resp.text != "" {
			respParts = append(respParts, resp.text)
		}
	}
	content := strings.Join(respParts, "\n\n")

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
			Breed:   breedIDs[0],
			Success: true,
			Steps:   make(map[string]protocol.StepResult),
			Content: content, // G9
		}))
	}
}

func (h *WSHandler) handleA2AHandoff(ctx context.Context, thread *a2a.Thread, fromBreed, fromVariant, toBreed, sessionID, invID, artifact string) {
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
	// G7+G12: enrich the artifact with the full context-transport envelope —
	// source notice (a2aFrom), continuity capsule, tombstone (on burst),
	// coverage map, importance anchors, and secret/tool-payload scrubbing —
	// before it crosses the breed boundary.
	enriched := buildEnrichedHandoffContext(HandoffTransportContext{
		FromBreed:    fromBreed,
		ToBreed:      toBreed,
		Artifact:     artifact,
		RecentBreeds: thread.Participants,
	})
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
		// G11: record this accepted handoff target + its source into the dynamic
		// worklist so the fan-out set stays accurate for later expansion dedup
		// (pushToWorklist's entry.list.push + a2aFrom parity).
		h.platform.Worklist.PushToWorklist(invID, []string{toBreed}, fromBreed)
	}
	toVariantID := defaultVariantID(h.platform, toBreed)
	h.platform.A2AHub.Handoff(ctx, thread, a2a.Handoff{FromBreed: fromBreed, FromVariant: fromVariant, ToBreed: toBreed, ToVariant: toVariantID, Artifact: enriched})
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
	// Enforce the cross-model review invariant: a dog may not hand its own
	// authored work to itself for review, and a review verdict may only be
	// written back by the assigned reviewer into the direct review thread.
	// This is the platform-level gate that keeps review independent of the
	// author; it fails closed. The author identity is the dog_id of the
	// variant that actually executed the breed (fromVariant), so an execution
	// that runs a non-default model variant resolves to a distinct reviewing
	// identity rather than collapsing onto the breed default.
	fromVariantID := fromVariant
	authorDog := dogIDFor(h.platform, fromBreed, fromVariantID)
	reviewerDog := dogIDFor(h.platform, toBreed, toVariantID)
	if verdict := h.platform.SOP.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed:       fromBreed,
		AuthorDogID:       authorDog,
		AuthorVariantID:   fromVariantID,
		ReviewerBreed:     toBreed,
		ReviewerDogID:     reviewerDog,
		ReviewerVariantID: toVariantID,
		SessionID:         sessionID,
	}); verdict.Blocked {
		h.SendSystemNotice("warn", "交接被拒：跨模型审查门禁", strings.Join(verdict.Messages, "; "))
		h.emitSopGate(ctx, sessionID, authorDog, reviewerDog, strings.Join(verdict.Messages, "; "), true)
		return
	}
	// A valid cross-model handoff: surface the routing so the operator can see
	// that independent verification is engaged.
	if reviewer := h.platform.SOP.SelectReviewer(fromBreed, thread.Participants, sopPorts.ReviewPolicy{RequireDifferentBreed: true}); reviewer != "" && reviewer != toBreed {
		h.emitSopGate(ctx, sessionID, authorDog, reviewerDog,
			fmt.Sprintf("跨模型审查已触发：建议由 %s 审查 %s 的工作，本次已路由至 %s。", reviewer, fromBreed, toBreed), false)
	}
	// §4.5 read-driven gate: before actually dispatching to the next breed
	// (which may be an external A2A agent), confirm via the custody ledger
	// projection that the ball is still live and that fromBreed handed it to
	// toBreed. This turns the ledger from write-only audit into a real
	// pre-dispatch guard against a concurrent/late handoff superseding this
	// one between G1 (above) and the actual send.
	if h.platform != nil && h.platform.BallLedger != nil {
		snap, snapErr := h.platform.BallLedger.Snapshot(ctx, sessionID)
		if snapErr == nil && snap.State != custodyPorts.BallStateVoid && snap.State != custodyPorts.BallStateDead {
			if snap.Holder != "" && snap.Holder != toBreed && snap.Holder != fromBreed {
				h.SendSystemNotice("warn", "传球中止", "球权在交接前已被 "+snap.Holder+" 接管，跳过本次到 "+toBreed+" 的 handoff。")
				return
			}
		}
	}
	h.executeWithPlatform(ctx, toBreed, sessionID, enriched, invID, false)
}

// dogIDFor resolves the canonical dog identity (the executing model). Given a
// breed and the executing variant ID, it returns that variant's dog_id; when
// the variant ID is empty or unknown it falls back to the breed's default
// variant, then to the breed dog_id. Review identity therefore follows the
// model that performs the work, so two executions that resolve to the same
// dog_id are the same reviewing identity regardless of breed label.
func dogIDFor(p *platform.Platform, breedID, variantID string) string {
	if b := p.GetBreed(breedID); b != nil {
		if variantID != "" {
			if v := b.VariantByID(variantID); v != nil && v.DogID != "" {
				return v.DogID
			}
		}
		if v := b.DefaultVariant(); v != nil && v.DogID != "" {
			return v.DogID
		}
		if b.DogID != "" {
			return b.DogID
		}
	}
	return ""
}

// defaultVariantID returns the ID of the variant that will execute a breed, or
// empty if the breed has no variant configured.
func defaultVariantID(p *platform.Platform, breedID string) string {
	if b := p.GetBreed(breedID); b != nil {
		if v := b.DefaultVariant(); v != nil {
			return v.ID
		}
	}
	return ""
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
