package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"sounds-great-ai/internal/adapter/unified"
	"sounds-great-ai/internal/aspect"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	routingPorts "sounds-great-ai/internal/domains/routing/ports"
	"sounds-great-ai/internal/platform"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const maxConcurrentBark = 8

type WSHandler struct {
	upgrader    websocket.Upgrader
	mu          sync.RWMutex
	streamers   map[string]*Streamer
	allConns    map[*websocket.Conn]struct{} // every live connection (global broadcast)
	pack        *pack.Pack
	platform    *platform.Platform // optional, nil = legacy mode
	sem         chan struct{}
	rateMonitor *RateMonitor
	// profiles optionally enables the on-session-seal autonomous distill
	// trigger (KD-10 maturity). Nil = disabled (no-op on seal).
	profiles *ProfilesHandler
	// escalations tracks pending CVO escalations (G4) by id so a
	// CVO_ESCALATION_RESPONSE can be resolved against the originating
	// thread and its decision options. Entries are dropped once answered.
	escalations map[string]*pendingEscalation
	// approvals owns the HITL approval loop: any aspect.RequestApproval
	// call blocks until the operator answers the HITL_APPROVAL card via
	// HITL_RESPONSE on this handler.
	approvals *aspect.ApprovalManager
	// notifications optionally mirrors every SendSystemNotice into the
	// notification center so the UI bell surfaces real data. Nil = off.
	notifications *NotificationsHandler
}

// pendingEscalation is a live CVO escalation awaiting an operator decision.
type pendingEscalation struct {
	SessionID string
	Payload   *protocol.CvoEscalationPayload
	CreatedAt time.Time
}

// SetProfilesHandler wires the capsule handler so session seal can fire a
// best-effort autonomous distill. Safe to call once at startup; nil disables.
func (h *WSHandler) SetProfilesHandler(p *ProfilesHandler) {
	h.profiles = p
}

// SetNotificationsHandler wires the notification center so SendSystemNotice
// also lands in /api/notifications. Safe to call once at startup; nil disables.
func (h *WSHandler) SetNotificationsHandler(n *NotificationsHandler) {
	h.notifications = n
}

func NewWSHandler(p *pack.Pack) *WSHandler {
	h := &WSHandler{
		upgrader:    newUpgrader(),
		streamers:   make(map[string]*Streamer),
		allConns:    make(map[*websocket.Conn]struct{}),
		escalations: make(map[string]*pendingEscalation),
		approvals:   aspect.NewApprovalManager(),
		pack:        p,
		sem:         make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(nil),
	}
	h.wireApprovalSender()
	return h
}

func newUpgrader() websocket.Upgrader {
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			if allowedOrigin == "" {
				return true
			}
			origin := r.Header.Get("Origin")
			return origin == "" || origin == allowedOrigin
		},
		EnableCompression: true,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
	}
}

// NewWSHandlerWithPlatform creates a WSHandler with platform adapter support.
// When platform is set, execution goes through CLI adapters instead of pack.Bark().
func NewWSHandlerWithPlatform(p *pack.Pack, pl *platform.Platform) *WSHandler {
	h := &WSHandler{
		upgrader:    newUpgrader(),
		streamers:   make(map[string]*Streamer),
		allConns:    make(map[*websocket.Conn]struct{}), // global broadcast registry (T25)
		escalations: make(map[string]*pendingEscalation),
		approvals:   aspect.NewApprovalManager(),
		pack:        p,
		platform:    pl,
		sem:         make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(func(sessionID string, count int) {
			log.Printf("WARN: broadcast rate exceeded for session %s: %d events/1s", sessionID, count)
		}),
	}
	h.wireApprovalSender()
	// G5: timed/command holds auto-resume their holder through this handler.
	if pl != nil && pl.HoldScheduler != nil {
		pl.HoldScheduler.SetOnWake(func(ctx context.Context, threadID, holder, resumeMsg string) {
			go h.resumeHeld(ctx, threadID, holder, resumeMsg)
		})
	}
	// T25: wire the carrier-health broadcaster so CARRIER_HEALTH events reach
	// every connected client. WSHandler implements unified.HealthBroadcaster.
	if pl != nil {
		pl.SetHealthBroadcaster(h)
	}
	return h
}

func (h *WSHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	h.addConn(conn)
	defer h.removeConn(conn)

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
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				errEvent := protocol.NewEvent("ERROR", "", map[string]string{
					"error": "invalid user input payload",
				})
				streamer.SendEvent(context.Background(), errEvent)
				continue
			}

			sessionID = payload.SessionID
			if sessionID == "" {
				sessionID = ev.SessionID
			}

			h.bindStreamer(sessionID, streamer)
			h.routeAndDispatch(streamer, sessionID, payload.Message)
		}

		if ev.Type == protocol.EventCvoEscalationResponse {
			var payload protocol.CvoEscalationResponsePayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				log.Printf("CVO_ESCALATION_RESPONSE parse failed: %v", err)
				continue
			}
			if payload.SessionID != "" {
				sessionID = payload.SessionID
			}
			h.handleEscalationResponse(streamer, sessionID, payload)
			continue
		}

		if ev.Type == protocol.EventHitlResponse {
			var payload protocol.HitlResponsePayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				log.Printf("HITL_RESPONSE parse failed: %v", err)
				continue
			}
			// Resolve the pending RequestApproval (aspect command guard /
			// governed tools): the blocked caller wakes with the decision.
			if h.approvals.ResolveApproval(payload.RequestID, payload.Approved) {
				log.Printf("HITL_RESPONSE resolved: request_id=%s approved=%v reason=%s", payload.RequestID, payload.Approved, payload.Reason)
			} else {
				log.Printf("HITL_RESPONSE for unknown/completed request_id=%s", payload.RequestID)
				h.SendSystemNotice("info", "审批状态", "未找到对应的待处理审批（可能已超时或已被处理）。")
			}
			continue
		}

		if ev.Type == protocol.EventWakeHold {
			var payload protocol.WakeHoldPayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				log.Printf("WAKE_HOLD parse failed: %v", err)
				continue
			}
			sessionID = payload.SessionID
			if sessionID == "" {
				sessionID = ev.SessionID
			}
			if h.platform == nil || h.platform.HoldScheduler == nil {
				log.Printf("WAKE_HOLD ignored: hold scheduler unavailable")
				continue
			}
			if err := h.ResumeHeldThread(context.Background(), sessionID, custodyPorts.WakeKind(payload.Kind), payload.Token); err != nil {
				log.Printf("WAKE_HOLD failed for %s: %v", sessionID, err)
				h.SendSystemNotice("warning", "唤醒失败", err.Error())
			}
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

// addConn registers a live connection for global broadcasts.
func (h *WSHandler) addConn(conn *websocket.Conn) {
	h.mu.Lock()
	h.allConns[conn] = struct{}{}
	h.mu.Unlock()
}

// removeConn deregisters a connection.
func (h *WSHandler) removeConn(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.allConns, conn)
	h.mu.Unlock()
}

// bindStreamer registers the connection's streamer under sessionID so later
// events (and escalations) on that thread reach this client.
func (h *WSHandler) bindStreamer(sessionID string, streamer *Streamer) {
	h.mu.Lock()
	h.streamers[sessionID] = streamer
	h.mu.Unlock()
}

// Approvals exposes the HITL approval manager so governed execution paths
// (aspect command guard, platform tools) can raise approvals that surface on
// this handler's connections.
func (h *WSHandler) Approvals() *aspect.ApprovalManager {
	return h.approvals
}

// wireApprovalSender routes aspect approval requests to the requesting
// session's streamer as HITL_APPROVAL events. Set once at construction —
// eventSender is never mutated afterwards.
func (h *WSHandler) wireApprovalSender() {
	h.approvals.SetEventSender(func(ctx context.Context, ev *protocol.Event) {
		if s := h.GetStreamer(ev.SessionID); s != nil {
			_ = s.SendEvent(ctx, ev)
		} else {
			log.Printf("HITL_APPROVAL for session %s has no live streamer; waiting for response anyway", ev.SessionID)
		}
	})
}

// PendingEscalationView is the REST projection of a pending CVO escalation,
// consumed by the frontend to re-hydrate escalation cards after a reload.
type PendingEscalationView struct {
	EscalationID string                         `json:"escalation_id"`
	SessionID    string                         `json:"session_id"`
	CreatedAt    time.Time                      `json:"created_at"`
	Payload      *protocol.CvoEscalationPayload `json:"payload"`
}

// PendingEscalations lists unresolved CVO escalations (oldest first). The
// registry is in-memory: entries survive page reloads but not server restarts.
func (h *WSHandler) PendingEscalations() []PendingEscalationView {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]PendingEscalationView, 0, len(h.escalations))
	for id, esc := range h.escalations {
		out = append(out, PendingEscalationView{
			EscalationID: id,
			SessionID:    esc.SessionID,
			CreatedAt:    esc.CreatedAt,
			Payload:      esc.Payload,
		})
	}
	// Deterministic order for stable UI lists.
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// routeAndDispatch routes a user-authored message to breed(s) and kicks off
// execution. It is the shared dispatch path for USER_INPUT messages and for
// prompts reborn from a resolved CVO escalation option.
func (h *WSHandler) routeAndDispatch(streamer *Streamer, sessionID, message string) {
	// Parse @mention to determine breed(s)
	var breedID string
	var routingDecision *routingPorts.RoutingDecision
	if h.platform != nil && h.platform.MentionRouter != nil {
		rd, _ := h.platform.MentionRouter.Route(context.Background(), message)
		routingDecision = &rd
		if len(rd.TargetBreeds) == 0 {
			// No available breeds (e.g. empty first-run catalog).
			// Surface a friendly error instead of routing to a
			// non-existent default breed that would fail at execution.
			warn := "无可用犬，请先在成员管理添加成员"
			if len(rd.Warnings) > 0 {
				warn = rd.Warnings[0]
			}
			streamer.SendEvent(context.Background(), protocol.NewEvent(protocol.EventBarkError, sessionID, &protocol.BarkErrorPayload{
				Breed: "",
				Error: warn,
			}))
			return
		}
		breedID = rd.TargetBreeds[0]
	} else {
		breedID = parseMention(message, h.pack)
	}

	// Push BARK_START immediately
	startEvent := protocol.NewEvent(protocol.EventBarkStart, sessionID, &protocol.BarkStartPayload{
		Breed:     breedID,
		SessionID: sessionID,
		Query:     message,
	})
	streamer.SendEvent(context.Background(), startEvent)

	// Run Bark in goroutine with session-scoped context
	go func(breedID, sessionID, query string, rd *routingPorts.RoutingDecision) {
		// Acquire semaphore
		h.sem <- struct{}{}
		defer func() { <-h.sem }()

		// Use context that survives client disconnect
		barkCtx := context.Background()
		// Mint one invocation id per user message so the A2A worklist
		// (depth + ping-pong) shares a single budget for the whole chain.
		invID := uuid.NewString()

		if h.platform != nil {
			if rd != nil && rd.Strategy == "serial" {
				h.executeSerial(barkCtx, rd.TargetBreeds, sessionID, query, invID)
			} else if rd != nil && rd.Strategy == "parallel" {
				h.executeParallel(barkCtx, rd.TargetBreeds, sessionID, query, invID)
			} else {
				h.executeWithPlatform(barkCtx, breedID, sessionID, query, invID, false)
			}
			if h.platform.Worklist != nil {
				h.platform.Worklist.Done(invID)
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
	}(breedID, sessionID, message, routingDecision)
}

// emitCvoEscalation registers and pushes a CVO_ESCALATION event for the
// session (G4). The operator answers via CVO_ESCALATION_RESPONSE; option
// labels are localized client-side so only semantic ids cross the wire.
func (h *WSHandler) emitCvoEscalation(sessionID, fromBreed, toBreed, reason string) {
	escID := uuid.NewString()
	payload := &protocol.CvoEscalationPayload{
		EscalationID: escID,
		Reason:       reason,
		FromBreed:    fromBreed,
		ToBreed:      toBreed,
		Options: []protocol.CvoEscalationOption{
			{
				ID:     "option_1",
				Prompt: "A2A 深度超限已熔断，请接手当前线程：总结各方进展与分歧，给出下一步拆解方案。",
			},
			{
				ID:     "option_2",
				Prompt: "A2A 深度超限已熔断，请停止本链工作，输出收尾总结与未竟事项清单。",
			},
		},
	}
	if h.platform != nil && h.platform.SOP != nil {
		payload.MaxDepth = h.platform.SOP.MaxA2ADepth()
	}
	h.mu.Lock()
	if h.escalations == nil {
		h.escalations = make(map[string]*pendingEscalation)
	}
	h.escalations[escID] = &pendingEscalation{SessionID: sessionID, Payload: payload, CreatedAt: time.Now()}
	h.mu.Unlock()
	if s := h.GetStreamer(sessionID); s != nil {
		_ = s.SendEvent(context.Background(), protocol.NewEvent(protocol.EventCvoEscalation, sessionID, payload))
	}
}

// takeEscalation atomically removes and returns a pending escalation.
func (h *WSHandler) takeEscalation(escalationID string) *pendingEscalation {
	h.mu.Lock()
	defer h.mu.Unlock()
	esc := h.escalations[escalationID]
	delete(h.escalations, escalationID)
	return esc
}

// handleEscalationResponse resolves a pending CVO escalation. When the
// chosen option carries a prompt, it is re-dispatched into the thread exactly
// like a user message; "intervene" (or an unknown decision) resolves without
// re-dispatch while the operator types a custom directive.
func (h *WSHandler) handleEscalationResponse(streamer *Streamer, sessionID string, payload protocol.CvoEscalationResponsePayload) {
	esc := h.takeEscalation(payload.EscalationID)
	if esc == nil {
		log.Printf("CVO_ESCALATION_RESPONSE for unknown/completed escalation %q", payload.EscalationID)
		h.SendSystemNotice("info", "升级状态", "未找到对应的待处理升级（可能已被处理或服务已重启）。")
		return
	}
	if sessionID == "" {
		sessionID = esc.SessionID
	}
	for _, opt := range esc.Payload.Options {
		if opt.ID != payload.Decision || opt.Prompt == "" {
			continue
		}
		h.bindStreamer(sessionID, streamer)
		h.SendSystemNotice("info", "升级已处理", "已按选项 "+payload.Decision+" 重新派发指令。")
		h.routeAndDispatch(streamer, sessionID, opt.Prompt)
		return
	}
	h.SendSystemNotice("info", "升级已处理", "升级已标记为人工介入，等待 CVO 手动指挥。")
}

// BroadcastCarrierHealth implements unified.HealthBroadcaster: it pushes a
// CARRIER_HEALTH event to every connected client so the frontend
// ConnectionStatusBar can render upstream model health directly (T25 / R6).
func (h *WSHandler) BroadcastCarrierHealth(_ context.Context, ev unified.CarrierHealthEvent) {
	event := protocol.NewEvent(protocol.EventCarrierHealth, "", &protocol.CarrierHealthPayload{
		Carrier:     ev.Carrier,
		Transport:   ev.Transport,
		Level:       ev.Level,
		Reason:      ev.Reason,
		RemainingMs: ev.RemainingMs,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.allConns {
		if err := conn.WriteJSON(event); err != nil {
			// A broken connection will be cleaned up by HandleWS's defer; do
			// not let one bad socket block the broadcast.
			continue
		}
	}
}
