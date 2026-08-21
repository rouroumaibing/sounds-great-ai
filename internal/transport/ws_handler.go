package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	routingPorts "sounds-great-ai/internal/domains/routing/ports"
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	"sounds-great-ai/internal/adapter/unified"
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
}

// SetProfilesHandler wires the capsule handler so session seal can fire a
// best-effort autonomous distill. Safe to call once at startup; nil disables.
func (h *WSHandler) SetProfilesHandler(p *ProfilesHandler) {
	h.profiles = p
}

func NewWSHandler(p *pack.Pack) *WSHandler {
	return &WSHandler{
		upgrader:    newUpgrader(),
		streamers:   make(map[string]*Streamer),
		allConns:    make(map[*websocket.Conn]struct{}),
		pack:        p,
		sem:         make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(nil),
	}
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
		pack:        p,
		platform:    pl,
		sem:         make(chan struct{}, maxConcurrentBark),
		rateMonitor: NewRateMonitor(func(sessionID string, count int) {
			log.Printf("WARN: broadcast rate exceeded for session %s: %d events/1s", sessionID, count)
		}),
	}
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

			h.mu.Lock()
			h.streamers[sessionID] = streamer
			h.mu.Unlock()

			// Parse @mention to determine breed(s)
			var breedID string
		var routingDecision *routingPorts.RoutingDecision
		if h.platform != nil && h.platform.MentionRouter != nil {
			rd, _ := h.platform.MentionRouter.Route(context.Background(), payload.Message)
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
					continue
				}
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
			}(breedID, sessionID, payload.Message, routingDecision)
		}

		if ev.Type == protocol.EventHitlResponse {
			var payload protocol.HitlResponsePayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				log.Printf("HITL_RESPONSE parse failed: %v", err)
				continue
			}
			log.Printf("HITL_RESPONSE received: request_id=%s approved=%v reason=%s", payload.RequestID, payload.Approved, payload.Reason)
			// TODO: Forward to agent/hitl channel when HITL flow is fully integrated
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
