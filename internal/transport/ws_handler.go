package transport

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"sounds-great-ai/pkg/pack"
	"sounds-great-ai/pkg/protocol"
)

const maxConcurrentBark = 8

type WSHandler struct {
	upgrader  websocket.Upgrader
	mu        sync.RWMutex
	streamers map[string]*Streamer
	pack      *pack.Pack
	sem       chan struct{}
}

func NewWSHandler(p *pack.Pack) *WSHandler {
	return &WSHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		streamers: make(map[string]*Streamer),
		pack:      p,
		sem:       make(chan struct{}, maxConcurrentBark),
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

			// Parse @mention to determine breed
			breedID := parseMention(payload.Message, h.pack)

			// Push BARK_START immediately
			startEvent := protocol.NewEvent(protocol.EventBarkStart, sessionID, &protocol.BarkStartPayload{
				Breed:     breedID,
				SessionID: sessionID,
				Query:     payload.Message,
			})
			streamer.SendEvent(context.Background(), startEvent)

			// Run Bark in goroutine with session-scoped context
			go func(breedID, sessionID, query string) {
				// Acquire semaphore
				h.sem <- struct{}{}
				defer func() { <-h.sem }()

				// Use context that survives client disconnect
				barkCtx := context.WithoutCancel(context.Background())

				input := &pack.TaskInput{
					Query: query,
					Context: &pack.ExecutionContext{
						SessionID: sessionID,
					},
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
			}(breedID, sessionID, payload.Message)
		}
	}

	if sessionID != "" {
		h.mu.Lock()
		delete(h.streamers, sessionID)
		h.mu.Unlock()
	}
}

func (h *WSHandler) GetStreamer(sessionID string) *Streamer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.streamers[sessionID]
}
