package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"sounds-great-ai/internal/platform"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"

	"github.com/gorilla/websocket"
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
		upgrader:    newUpgrader(),
		streamers:   make(map[string]*Streamer),
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
	return &WSHandler{
		upgrader:    newUpgrader(),
		streamers:   make(map[string]*Streamer),
		pack:        p,
		platform:    pl,
		sem:         make(chan struct{}, maxConcurrentBark),
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
				barkCtx := context.Background()

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
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				log.Printf("HITL_RESPONSE parse failed: %v", err)
				continue
			}
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
