package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/config"
	"sounds-great-ai/internal/settings"
)

func setupConfigHandler(t *testing.T) *ConfigHandler {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"id":"test","name":"Test","display_name":"Test","personality":"test","variants":[{"id":"default","client_id":"anthropic","default_model":"claude"}]}`), 0644)
	loader := config.NewLoader()
	store := settings.NewInMemorySettingsStore()
	envPath := filepath.Join(dir, ".env")
	return NewConfigHandler(loader, dir, store, envPath)
}

func TestGetDefaultBreed(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/default-breed", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGetBreedOrder(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/breed-order", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestUpdateBreedOrder(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	body, _ := json.Marshal(map[string][]string{"order": {"test"}})
	req := httptest.NewRequest("PUT", "/api/config/breed-order", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetEnvSummary(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/env-summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["data_dirs"] == nil {
		t.Fatal("data_dirs should be present")
	}
}

func TestGetLeaderDefault(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/config/leader", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["name"] != "You" {
		t.Errorf("name = %v, want You", result["name"])
	}
}

func TestUpdateLeader(t *testing.T) {
	h := setupConfigHandler(t)
	leader := config.DefaultLeaderConfig()
	h.SetLeader(&leader)
	mux := h.Routes()

	body, _ := json.Marshal(map[string]any{
		"name":            "Operator",
		"aliases":         []string{"boss"},
		"mentionPatterns": []string{"@leader", "@op"},
		"timeZone":        "UTC",
	})
	req := httptest.NewRequest("PATCH", "/api/config/leader", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)
	if result["name"] != "Operator" {
		t.Errorf("name = %v, want Operator", result["name"])
	}
}

func TestUpdateLeaderInvalid(t *testing.T) {
	h := setupConfigHandler(t)
	mux := h.Routes()

	body, _ := json.Marshal(map[string]any{
		"name":            "",
		"mentionPatterns": []string{"@leader"},
	})
	req := httptest.NewRequest("PATCH", "/api/config/leader", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
