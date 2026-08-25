package transport

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"sounds-great-ai/internal/domains/threads"
	threadPorts "sounds-great-ai/internal/domains/threads/ports"
)

// ThreadHandler handles thread + session + message HTTP endpoints.
type ThreadHandler struct {
	store        threadPorts.IThreadStore
	messageStore threadPorts.IMessageStore // optional, nil = message endpoints disabled
	inbox        *threads.Inbox            // optional, nil = delivery-status endpoints disabled
}

// NewThreadHandler creates a new ThreadHandler.
func NewThreadHandler(store threadPorts.IThreadStore) *ThreadHandler {
	return &ThreadHandler{store: store}
}

// NewThreadHandlerWithMessages creates a ThreadHandler with message store support.
func NewThreadHandlerWithMessages(store threadPorts.IThreadStore, ms threadPorts.IMessageStore) *ThreadHandler {
	return &ThreadHandler{store: store, messageStore: ms}
}

// NewThreadHandlerWithInbox creates a ThreadHandler with delivery-status (inbox)
// support layered on top of the thread store. The inbox powers the message /
// delivery / branch read-model (README 十大缺口 #1).
func NewThreadHandlerWithInbox(store threadPorts.IThreadStore, inbox *threads.Inbox) *ThreadHandler {
	return &ThreadHandler{store: store, inbox: inbox}
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
	mux.HandleFunc("POST /api/threads/{id}/messages", h.PostMessage)
	mux.HandleFunc("POST /api/threads/{id}/events", h.AddThreadEvent)
	mux.HandleFunc("GET /api/threads/{id}/sessions", h.ListSessions)
	mux.HandleFunc("POST /api/sessions/{id}/unseal", h.UnsealSession)
	// Delivery-status (inbox) endpoints — P0-1 message/delivery substrate.
	mux.HandleFunc("GET /api/threads/{id}/inbox", h.GetInbox)
	mux.HandleFunc("POST /api/threads/{id}/inbox", h.ReceiveInboxMessage)
	mux.HandleFunc("PUT /api/threads/{id}/messages/{mid}/delivery", h.SetDeliveryStatus)
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
		// The message store is wired by the platform at startup. If it is absent
		// the server is running in legacy/reduced mode and history is genuinely
		// unavailable; report that honestly rather than masking it as empty.
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

// PostMessage handles POST /api/threads/{id}/messages — append a message to a
// thread. This is the REST counterpart of the WebSocket message flow, exposed
// so the platform MCP server (and other non-WS clients) can post messages as a
// first-class platform capability. role defaults to "user" and sender to
// "mcp" when omitted.
func (h *ThreadHandler) PostMessage(w http.ResponseWriter, r *http.Request) {
	if h.messageStore == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "message store not configured"})
		return
	}
	threadID := r.PathValue("id")
	var body struct {
		Content string `json:"content"`
		Role    string `json:"role"`
		Sender  string `json:"sender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	role := body.Role
	if role == "" {
		role = "user"
	}
	sender := body.Sender
	if sender == "" {
		sender = "mcp"
	}
	msg := &threadPorts.Message{
		ID:        uuid.NewString(),
		ThreadID:  threadID,
		Role:      role,
		Content:   content,
		Sender:    sender,
		Timestamp: time.Now(),
	}
	if err := h.messageStore.Append(msg); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, msg)
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

// GetInbox handles GET /api/threads/{id}/inbox — list a thread's messages with
// their delivery status (the inbox read-model). Disabled when no inbox is wired.
func (h *ThreadHandler) GetInbox(w http.ResponseWriter, r *http.Request) {
	if h.inbox == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "inbox not configured"})
		return
	}
	threadID := r.PathValue("id")
	msgs := h.inbox.MessagesForThread(threadID, false)
	respondJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// ReceiveInboxMessage handles POST /api/threads/{id}/inbox — receive a message
// into the inbox. Idempotent on message_id: re-receiving the same id is a no-op
// (returns the existing record with 200 and "idempotent": true).
func (h *ThreadHandler) ReceiveInboxMessage(w http.ResponseWriter, r *http.Request) {
	if h.inbox == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "inbox not configured"})
		return
	}
	threadID := r.PathValue("id")
	var body struct {
		ID      string `json:"id"`
		Sender  string `json:"sender"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.ID == "" {
		body.ID = uuid.NewString()
	}
	if strings.TrimSpace(body.Content) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	msg := threads.InboxMessage{
		ID:       body.ID,
		ThreadID: threadID,
		Sender:   body.Sender,
		Content:  body.Content,
	}
	already, err := h.inbox.Receive(msg)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	got, _ := h.inbox.Get(body.ID)
	respondJSON(w, http.StatusOK, map[string]any{"idempotent": already, "message": got})
}

// SetDeliveryStatus handles PUT /api/threads/{id}/messages/{mid}/delivery —
// transition a message's delivery status. Enforces the delivery state machine,
// so e.g. re-delivering a canceled message is rejected (409).
func (h *ThreadHandler) SetDeliveryStatus(w http.ResponseWriter, r *http.Request) {
	if h.inbox == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{"error": "inbox not configured"})
		return
	}
	mid := r.PathValue("mid")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	status := threads.DeliveryStatus(body.Status)
	if _, ok := h.inbox.Get(mid); !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "unknown message_id"})
		return
	}
	if err := h.inbox.SetDeliveryStatus(mid, status); err != nil {
		// SetDeliveryStatus enforces the state machine, so any error here is an
		// illegal transition (e.g. re-delivering a canceled message).
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	got, _ := h.inbox.Get(mid)
	respondJSON(w, http.StatusOK, map[string]any{"message": got})
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
