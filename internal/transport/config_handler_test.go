package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sounds-great-ai/internal/settings"

	"sounds-great-ai/pkg/pack"
)

func setupConfigHandler(t *testing.T) *ConfigHandler {
	t.Helper()
	dir := t.TempDir()
	// Write the consolidated template file (single-file model).
	breedJSON := `{"id":"test","name":"Test","display_name":"Test","personality":"test","variants":[{"id":"default","client_id":"anthropic","default_model":"claude"}]}`
	os.WriteFile(filepath.Join(dir, "dog-template.json"), []byte(`{"version":2,"breeds":[`+breedJSON+`]}`), 0644)
	loader := pack.NewLoader()
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
	leader := pack.DefaultLeaderConfig()
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

func TestSetBreedOrderReordersCatalog(t *testing.T) {
	dir := t.TempDir()
	breedJSON := `{"id":"a","name":"A","variants":[{"id":"default","client_id":"claude"}]}`
	os.WriteFile(filepath.Join(dir, "dog-template.json"), []byte(`{"version":2,"breeds":[`+breedJSON+`]}`), 0644)
	loader := pack.NewLoader()
	store := settings.NewInMemorySettingsStore()
	// Seed catalog with three breeds in a known order.
	for _, id := range []string{"x", "a", "y"} {
		if err := store.CreateBreed(&pack.BreedConfig{
			ID: id, Name: id, DefaultVariantID: "default",
			Variants: []pack.Variant{{ID: "default", ClientID: "claude"}},
			Source:  pack.BreedSourceUser,
		}); err != nil {
			t.Fatalf("seed breed %s: %v", id, err)
		}
	}
	h := NewConfigHandler(loader, dir, store, filepath.Join(dir, ".env"))
	mux := h.Routes()

	// Reorder to y, x, a.
	body, _ := json.Marshal(map[string][]string{"order": {"y", "x", "a"}})
	req := httptest.NewRequest("PUT", "/api/config/breed-order", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set order: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	// GET reflects the new persisted order.
	req2 := httptest.NewRequest("GET", "/api/config/breed-order", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get order: want 200, got %d", rec2.Code)
	}
	var out struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(rec2.Body).Decode(&out); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	want := []string{"y", "x", "a"}
	if len(out.Order) != len(want) {
		t.Fatalf("order len: want %d, got %d (%v)", len(want), len(out.Order), out.Order)
	}
	for i := range want {
		if out.Order[i] != want[i] {
			t.Fatalf("order[%d]: want %q, got %q", i, want[i], out.Order[i])
		}
	}
}

func TestSetDefaultBreedValidatesMergedBreeds(t *testing.T) {
	dir := t.TempDir()
	breedJSON := `{"id":"template-dog","name":"Template","variants":[{"id":"default","client_id":"claude"}]}`
	os.WriteFile(filepath.Join(dir, "dog-template.json"), []byte(`{"version":2,"breeds":[`+breedJSON+`]}`), 0644)
	loader := pack.NewLoader()
	store := settings.NewInMemorySettingsStore()
	// A runtime (catalog-only) breed must be selectable as the default.
	if err := store.CreateBreed(&pack.BreedConfig{
		ID: "runtime-dog", Name: "Runtime", DefaultVariantID: "default",
		Variants: []pack.Variant{{ID: "default", ClientID: "claude"}},
		Source:  pack.BreedSourceUser,
	}); err != nil {
		t.Fatalf("create breed: %v", err)
	}
	h := NewConfigHandler(loader, dir, store, filepath.Join(dir, ".env"))
	mux := h.Routes()

	// Valid: runtime breed is in the merged (template + catalog) set.
	body, _ := json.Marshal(map[string]string{"breed_id": "runtime-dog"})
	req := httptest.NewRequest("PUT", "/api/config/default-breed", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set runtime default: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	// GET returns the persisted value (restart-safe via the settings store).
	req2 := httptest.NewRequest("GET", "/api/config/default-breed", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	var got struct {
		BreedID string `json:"breed_id"`
	}
	if err := json.NewDecoder(rec2.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.BreedID != "runtime-dog" {
		t.Fatalf("default breed: want runtime-dog, got %q", got.BreedID)
	}

	// Unknown breed → 404.
	body2, _ := json.Marshal(map[string]string{"breed_id": "ghost"})
	req3 := httptest.NewRequest("PUT", "/api/config/default-breed", bytes.NewReader(body2))
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("set unknown default: want 404, got %d", rec3.Code)
	}
}
