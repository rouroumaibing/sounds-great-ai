package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"sounds-great-ai/internal/threadstore"
	threadStores "sounds-great-ai/internal/domains/threads/stores"
)

func setupTestHandler(t *testing.T) (*ThreadHandler, string) {
	t.Helper()
	ts := threadstore.NewInMemoryThreadStore()
	ms := threadstore.NewMemoryMessageStore()
	h := NewThreadHandlerWithMessages(threadStores.NewThreadStoreAdapter(ts), threadStores.NewMessageStoreAdapter(ms))

	thread, err := ts.CreateThread("Test Thread")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Add test messages with increasing timestamps (a=oldest, e=newest)
	for i := 0; i < 5; i++ {
		ms.Append(&threadstore.Message{
			ID:        "msg-" + string(rune('a'+i)),
			ThreadID:  thread.ID,
			Role:      "user",
			Content:   "message " + string(rune('0'+i+1)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	return h, thread.ID
}

func TestListMessages_NoCursor(t *testing.T) {
	h, threadID := setupTestHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Messages []*threadstore.Message `json:"messages"`
		HasMore  bool                   `json:"has_more"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Messages) != 5 {
		t.Fatalf("got %d messages, want 5", len(resp.Messages))
	}
	if resp.HasMore {
		t.Fatal("has_more should be false")
	}
}

func TestListMessages_WithLimit(t *testing.T) {
	h, threadID := setupTestHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages?limit=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp struct {
		Messages []*threadstore.Message `json:"messages"`
		HasMore  bool                   `json:"has_more"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(resp.Messages))
	}
	if !resp.HasMore {
		t.Fatal("has_more should be true")
	}
}

func TestListMessages_WithCursor(t *testing.T) {
	h, threadID := setupTestHandler(t)
	mux := h.Routes()

	// First page: get 2 most recent (msg-d, msg-e)
	req1 := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages?limit=2", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)

	var resp1 struct {
		Messages []*threadstore.Message `json:"messages"`
		HasMore  bool                   `json:"has_more"`
	}
	json.NewDecoder(rec1.Body).Decode(&resp1)

	if len(resp1.Messages) != 2 {
		t.Fatalf("page 1: got %d messages, want 2", len(resp1.Messages))
	}
	if !resp1.HasMore {
		t.Fatal("page 1: has_more should be true")
	}
	// Ascending order: oldest first → msg-d, msg-e
	if resp1.Messages[0].ID != "msg-d" || resp1.Messages[1].ID != "msg-e" {
		t.Fatalf("page 1: got %s, %s; want msg-d, msg-e", resp1.Messages[0].ID, resp1.Messages[1].ID)
	}

	// Build cursor from oldest message in page 1 (msg-d)
	oldest := resp1.Messages[0]
	cursor := strconv.FormatInt(oldest.Timestamp.UnixNano(), 10) + ":" + oldest.ID

	// Second page: messages before msg-d → msg-b, msg-c
	req2 := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages?limit=2&before="+cursor, nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	var resp2 struct {
		Messages []*threadstore.Message `json:"messages"`
		HasMore  bool                   `json:"has_more"`
	}
	json.NewDecoder(rec2.Body).Decode(&resp2)

	if len(resp2.Messages) != 2 {
		t.Fatalf("page 2: got %d messages, want 2", len(resp2.Messages))
	}
	if !resp2.HasMore {
		t.Fatal("page 2: has_more should be true")
	}
	if resp2.Messages[0].ID != "msg-b" || resp2.Messages[1].ID != "msg-c" {
		t.Fatalf("page 2: got %s, %s; want msg-b, msg-c", resp2.Messages[0].ID, resp2.Messages[1].ID)
	}

	// Build cursor from oldest in page 2 (msg-b)
	oldest2 := resp2.Messages[0]
	cursor2 := strconv.FormatInt(oldest2.Timestamp.UnixNano(), 10) + ":" + oldest2.ID

	// Third page: messages before msg-b → msg-a only
	req3 := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages?limit=2&before="+cursor2, nil)
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)

	var resp3 struct {
		Messages []*threadstore.Message `json:"messages"`
		HasMore  bool                   `json:"has_more"`
	}
	json.NewDecoder(rec3.Body).Decode(&resp3)

	if len(resp3.Messages) != 1 {
		t.Fatalf("page 3: got %d messages, want 1", len(resp3.Messages))
	}
	if resp3.HasMore {
		t.Fatal("page 3: has_more should be false")
	}
	if resp3.Messages[0].ID != "msg-a" {
		t.Fatalf("page 3: got %s; want msg-a", resp3.Messages[0].ID)
	}
}

func TestListMessages_NoMessageStore(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	h := NewThreadHandler(threadStores.NewThreadStoreAdapter(ts)) // no message store
	mux := h.Routes()

	thread, _ := ts.CreateThread("Test")
	req := httptest.NewRequest("GET", "/api/threads/"+thread.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
}

func TestUpdateThread(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	ms := threadstore.NewMemoryMessageStore()
	h := NewThreadHandlerWithMessages(threadStores.NewThreadStoreAdapter(ts), threadStores.NewMessageStoreAdapter(ms))
	mux := h.Routes()

	thread, err := ts.CreateThread("Original Title")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Valid update
	body, _ := json.Marshal(map[string]string{"title": "Updated Title"})
	req := httptest.NewRequest("PATCH", "/api/threads/"+thread.ID, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Title != "Updated Title" {
		t.Fatalf("got title %q, want %q", resp.Title, "Updated Title")
	}

	// Verify persistence
	threads, _ := ts.ListThreads()
	for _, th := range threads {
		if th.ID == thread.ID && th.Title == "Updated Title" {
			return // success
		}
	}
	t.Fatal("title not persisted in store")
}

func TestUpdateThread_Validation(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	h := NewThreadHandler(threadStores.NewThreadStoreAdapter(ts))
	mux := h.Routes()

	thread, _ := ts.CreateThread("Original")

	// Empty title
	body, _ := json.Marshal(map[string]string{"title": ""})
	req := httptest.NewRequest("PATCH", "/api/threads/"+thread.ID, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty title: status = %d, want 400", rec.Code)
	}

	// Title too long (>200 chars)
	longTitle := strings.Repeat("x", 201)
	body, _ = json.Marshal(map[string]string{"title": longTitle})
	req = httptest.NewRequest("PATCH", "/api/threads/"+thread.ID, bytes.NewReader(body))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("long title: status = %d, want 400", rec.Code)
	}
}

func TestAddThreadEvent(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	h := NewThreadHandler(threadStores.NewThreadStoreAdapter(ts))
	mux := h.Routes()

	thread, err := ts.CreateThread("Test Thread")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	eventJSON := `{"type":"tool_call","tool":"git","status":"success"}`
	req := httptest.NewRequest("POST", "/api/threads/"+thread.ID+"/events", strings.NewReader(eventJSON))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}

	var resp struct {
		OK bool `json:"ok"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.OK {
		t.Fatal("ok should be true")
	}

	// Verify event was stored
	events, err := ts.GetEvents(thread.ID)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
}

func TestAddThreadEvent_Validation(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	h := NewThreadHandler(threadStores.NewThreadStoreAdapter(ts))
	mux := h.Routes()

	thread, _ := ts.CreateThread("Test")

	// Empty body
	req := httptest.NewRequest("POST", "/api/threads/"+thread.ID+"/events", strings.NewReader(""))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body: status = %d, want 400", rec.Code)
	}

	// Invalid JSON
	req = httptest.NewRequest("POST", "/api/threads/"+thread.ID+"/events", strings.NewReader("{bad json"))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON: status = %d, want 400", rec.Code)
	}
}

func TestPostMessage(t *testing.T) {
	h, threadID := setupTestHandler(t)
	mux := h.Routes()

	body, _ := json.Marshal(map[string]string{"content": "hello from mcp", "role": "user", "sender": "mcp"})
	req := httptest.NewRequest("POST", "/api/threads/"+threadID+"/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var resp threadstore.Message
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Content != "hello from mcp" {
		t.Fatalf("content = %q, want %q", resp.Content, "hello from mcp")
	}
	if resp.Role != "user" || resp.Sender != "mcp" {
		t.Fatalf("role/sender = %q/%q", resp.Role, resp.Sender)
	}
	if resp.ID == "" {
		t.Fatal("expected generated message id")
	}

	// The posted message must appear in the thread's message history.
	listReq := httptest.NewRequest("GET", "/api/threads/"+threadID+"/messages", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	var listResp struct {
		Messages []*threadstore.Message `json:"messages"`
	}
	json.NewDecoder(listRec.Body).Decode(&listResp)
	found := false
	for _, m := range listResp.Messages {
		if m.ID == resp.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("posted message not found in thread list")
	}
}

func TestPostMessage_DefaultsAndMissingContent(t *testing.T) {
	h, threadID := setupTestHandler(t)
	mux := h.Routes()

	// Defaults: role=user, sender=mcp when omitted, content required.
	body, _ := json.Marshal(map[string]string{"content": "plain"})
	req := httptest.NewRequest("POST", "/api/threads/"+threadID+"/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var resp threadstore.Message
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Role != "user" || resp.Sender != "mcp" {
		t.Fatalf("defaults: role/sender = %q/%q, want user/mcp", resp.Role, resp.Sender)
	}

	// Missing content → 400.
	bad, _ := json.Marshal(map[string]string{"role": "user"})
	req2 := httptest.NewRequest("POST", "/api/threads/"+threadID+"/messages", bytes.NewReader(bad))
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("missing content status = %d, want 400", rec2.Code)
	}
}

func TestPostMessage_NoMessageStore(t *testing.T) {
	ts := threadstore.NewInMemoryThreadStore()
	h := NewThreadHandler(threadStores.NewThreadStoreAdapter(ts)) // no message store
	mux := h.Routes()
	thread, _ := ts.CreateThread("Test")
	body, _ := json.Marshal(map[string]string{"content": "x"})
	req := httptest.NewRequest("POST", "/api/threads/"+thread.ID+"/messages", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rec.Code)
	}
}
