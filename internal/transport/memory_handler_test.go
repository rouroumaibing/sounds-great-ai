package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sounds-great-ai/internal/memory"
)

func TestMemoryHandler_ListEvidence_Empty(t *testing.T) {
	store := memory.NewEvidenceStore()
	h := NewMemoryHandler(store)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/memory/evidence", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []*memory.EvidenceRecord
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty list, got %d items", len(result))
	}
}

func TestMemoryHandler_AddEvidence(t *testing.T) {
	store := memory.NewEvidenceStore()
	h := NewMemoryHandler(store)
	mux := h.Routes()

	body, _ := json.Marshal(map[string]any{
		"content":   "test evidence content",
		"type":      "evidence",
		"title":     "Test Title",
		"thread_id": "thread-1",
		"tags":      []string{"tag1", "tag2"},
	})
	req := httptest.NewRequest("POST", "/api/memory/evidence", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var rec2 memory.EvidenceRecord
	if err := json.NewDecoder(rec.Body).Decode(&rec2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec2.Title != "Test Title" {
		t.Errorf("title = %s, want Test Title", rec2.Title)
	}
	if rec2.Content != "test evidence content" {
		t.Errorf("content = %s, want test evidence content", rec2.Content)
	}
}

func TestMemoryHandler_AddEvidence_ThenList(t *testing.T) {
	store := memory.NewEvidenceStore()
	h := NewMemoryHandler(store)
	mux := h.Routes()

	body, _ := json.Marshal(map[string]any{
		"content":   "persisted evidence",
		"type":      "evidence",
		"title":     "Persisted",
		"thread_id": "thread-2",
		"tags":      []string{"a"},
	})
	postReq := httptest.NewRequest("POST", "/api/memory/evidence", bytes.NewReader(body))
	postRec := httptest.NewRecorder()
	mux.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusCreated {
		t.Fatalf("post status = %d, want 201", postRec.Code)
	}

	getReq := httptest.NewRequest("GET", "/api/memory/evidence", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRec.Code)
	}
	var list []*memory.EvidenceRecord
	if err := json.NewDecoder(getRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(list))
	}
	if list[0].Title != "Persisted" {
		t.Errorf("title = %s, want Persisted", list[0].Title)
	}
}

func TestMemoryHandler_AddEvidence_InvalidBody(t *testing.T) {
	store := memory.NewEvidenceStore()
	h := NewMemoryHandler(store)
	mux := h.Routes()

	req := httptest.NewRequest("POST", "/api/memory/evidence", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
