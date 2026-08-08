package transport

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Notification represents a system notification.
type Notification struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
	Read      bool   `json:"read"`
}

// NotificationsHandler provides an in-memory notification store.
type NotificationsHandler struct {
	mu  sync.RWMutex
	all []Notification
}

func NewNotificationsHandler() *NotificationsHandler {
	return &NotificationsHandler{all: []Notification{}}
}

// Push appends a notification (thread-safe).
func (h *NotificationsHandler) Push(n Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if n.Timestamp == "" {
		n.Timestamp = time.Now().Format("15:04:05")
	}
	h.all = append(h.all, n)
}

func (h *NotificationsHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/notifications", h.handleCollection)
	mux.HandleFunc("/api/notifications/", h.handleItem)
	return mux
}

func (h *NotificationsHandler) handleCollection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.mu.RLock()
		defer h.mu.RUnlock()
		respondJSON(w, http.StatusOK, h.all)
	case http.MethodDelete:
		h.mu.Lock()
		h.all = h.all[:0]
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *NotificationsHandler) handleItem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[1] != "read" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.all {
		if h.all[i].ID == id {
			h.all[i].Read = true
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ensure notifications JSON encodes as [] not null
func init() {
	_ = json.Marshal
}
