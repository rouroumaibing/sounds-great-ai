package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPanelsHandler_Concierge(t *testing.T) {
	h := NewPanelsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/concierge", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["avatar"] != "🐕" {
		t.Errorf("avatar = %v, want 🐕", result["avatar"])
	}
}

func TestPanelsHandler_Voice(t *testing.T) {
	h := NewPanelsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/voice", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["enabled"] != false {
		t.Errorf("enabled = %v, want false", result["enabled"])
	}
}

func TestPanelsHandler_Connectors(t *testing.T) {
	h := NewPanelsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/connectors", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil connectors list")
	}
}

func TestPanelsHandler_Plugins(t *testing.T) {
	h := NewPanelsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/plugins", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil plugins list")
	}
}

func TestPanelsHandler_Marketplace(t *testing.T) {
	h := NewPanelsHandler()
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/marketplace", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil marketplace list")
	}
}
