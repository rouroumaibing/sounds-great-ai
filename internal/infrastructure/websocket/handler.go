package websocket

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

// UpgraderFactory creates a WebSocket upgrader with origin checking.
func NewUpgrader() websocket.Upgrader {
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

// SessionRegistry tracks active WebSocket sessions by session ID.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]bool
}

// NewSessionRegistry creates a new SessionRegistry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]bool)}
}

// Register adds a session to the registry.
func (r *SessionRegistry) Register(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionID] = true
}

// Unregister removes a session from the registry.
func (r *SessionRegistry) Unregister(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}

// Count returns the number of active sessions.
func (r *SessionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// LogUpgradeFailure logs a WebSocket upgrade failure.
func LogUpgradeFailure(err error) {
	log.Printf("WS upgrade failed: %v", err)
}
