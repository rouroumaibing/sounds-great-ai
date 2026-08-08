package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationsHandler_ListEmpty(t *testing.T) {
	h := NewNotificationsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/notifications", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []Notification
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty list, got %d items", len(result))
	}
}

func TestNotificationsHandler_PushThenList(t *testing.T) {
	h := NewNotificationsHandler()
	mux := h.Routes()

	h.Push(Notification{
		ID:       "n1",
		Severity: "info",
		Title:    "Test Notification",
		Message:  "Hello world",
		Source:   "test",
	})
	h.Push(Notification{
		ID:       "n2",
		Severity: "warning",
		Title:    "Another",
		Message:  "Second message",
		Source:   "test",
	})

	req := httptest.NewRequest("GET", "/api/notifications", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []Notification
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(result))
	}
	if result[0].ID != "n1" {
		t.Errorf("first id = %s, want n1", result[0].ID)
	}
	if result[1].ID != "n2" {
		t.Errorf("second id = %s, want n2", result[1].ID)
	}
}

func TestNotificationsHandler_DeleteClears(t *testing.T) {
	h := NewNotificationsHandler()
	mux := h.Routes()

	h.Push(Notification{ID: "n1", Title: "A", Message: "a", Source: "test"})
	h.Push(Notification{ID: "n2", Title: "B", Message: "b", Source: "test"})

	delReq := httptest.NewRequest("DELETE", "/api/notifications", nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", delRec.Code)
	}

	getReq := httptest.NewRequest("GET", "/api/notifications", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRec.Code)
	}
	var result []Notification
	if err := json.NewDecoder(getRec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty list after delete, got %d items", len(result))
	}
}

func TestNotificationsHandler_Options(t *testing.T) {
	h := NewNotificationsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("OPTIONS", "/api/notifications", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
