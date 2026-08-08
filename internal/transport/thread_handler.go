package transport

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sounds-great-ai/internal/threadstore"
)

// ThreadHandler handles thread + session + message HTTP endpoints.
type ThreadHandler struct {
	store        threadstore.ThreadStore
	messageStore threadstore.MessageStore // optional, nil = message endpoints disabled
}

// NewThreadHandler creates a new ThreadHandler.
func NewThreadHandler(store threadstore.ThreadStore) *ThreadHandler {
	return &ThreadHandler{store: store}
}

// NewThreadHandlerWithMessages creates a ThreadHandler with message store support.
func NewThreadHandlerWithMessages(store threadstore.ThreadStore, ms threadstore.MessageStore) *ThreadHandler {
	return &ThreadHandler{store: store, messageStore: ms}
}

// Routes returns the HTTP routes for threads + sessions + messages.
func (h *ThreadHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/threads", h.ListThreads)
	mux.HandleFunc("POST /api/threads", h.CreateThread)
	mux.HandleFunc("GET /api/threads/{id}", h.GetThread)
	mux.HandleFunc("PATCH /api/threads/{id}", h.UpdateThread)
	mux.HandleFunc("DELETE /api/threads/{id}", h.DeleteThread)
	mux.HandleFunc("GET /api/threads/{id}/messages", h.ListMessages)
	mux.HandleFunc("POST /api/threads/{id}/events", h.AddThreadEvent)
	mux.HandleFunc("GET /api/threads/{id}/sessions", h.ListSessions)
	mux.HandleFunc("POST /api/sessions/{id}/unseal", h.UnsealSession)
	return mux
}

func (h *ThreadHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	threads, err := h.store.ListThreads()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, threads)
}

func (h *ThreadHandler) CreateThread(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	thread, err := h.store.CreateThread(body.Title)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, thread)
}

func (h *ThreadHandler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteThread(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, nil)
}

func (h *ThreadHandler) GetThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	events, err := h.store.GetEvents(id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *ThreadHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("id")
	sessions, err := h.store.ListSessions(threadID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, sessions)
}

func (h *ThreadHandler) UnsealSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.UnsealSession(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, nil)
}

// UpdateThread handles PATCH /api/threads/{id} — update thread title.
func (h *ThreadHandler) UpdateThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	title := strings.TrimSpace(body.Title)
	if len(title) == 0 || len(title) > 200 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "title must be 1-200 characters"})
		return
	}
	if err := h.store.UpdateTitle(id, title); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"id": id, "title": title})
}

// ListMessages handles GET /api/threads/{id}/messages — cursor-paginated message history.
func (h *ThreadHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	if h.messageStore == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "message store not configured"})
		return
	}
	threadID := r.PathValue("id")

	// Parse limit (default 50, max 200)
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
			if limit > 200 {
				limit = 200
			}
		}
	}

	// Parse cursor "timestamp:id"
	var before time.Time
	var beforeID string
	if cursor := r.URL.Query().Get("before"); cursor != "" {
		if idx := strings.Index(cursor, ":"); idx > 0 {
			tsStr := cursor[:idx]
			beforeID = cursor[idx+1:]
			if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
				before = time.Unix(0, ts)
			}
		}
	}

	msgs, err := h.messageStore.GetByThreadBefore(threadID, before, beforeID, limit+1)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	hasMore := len(msgs) > limit
	if hasMore {
		// Messages are in ascending order (oldest first); keep the most recent `limit`.
		msgs = msgs[len(msgs)-limit:]
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"messages": msgs,
		"has_more": hasMore,
	})
}

// AddThreadEvent handles POST /api/threads/{id}/events — add custom event to thread.
func (h *ThreadHandler) AddThreadEvent(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("id")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	if len(body) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "empty body"})
		return
	}
	// Validate JSON
	if !json.Valid(body) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := h.store.AddEvent(threadID, json.RawMessage(body)); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
