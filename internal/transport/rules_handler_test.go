package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestRulesHandler_GetRules(t *testing.T) {
	loader := pack.NewLoader()
	h := NewRulesHandler(nil, loader, "", "")
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/rules", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ironLaws, ok := result["iron_laws"].([]any)
	if !ok || len(ironLaws) != 5 {
		t.Fatalf("iron_laws: want 5 items, got %v", result["iron_laws"])
	}
	redFlags, ok := result["red_flags"].([]any)
	if !ok || len(redFlags) != 4 {
		t.Fatalf("red_flags: want 4 items, got %v", result["red_flags"])
	}
	breedRestrictions, ok := result["breed_restrictions"].([]any)
	if !ok || len(breedRestrictions) != 6 {
		t.Fatalf("breed_restrictions: want 6 items, got %v", result["breed_restrictions"])
	}
}

func TestRulesHandler_GetHookManifest_NilRegistry(t *testing.T) {
	loader := pack.NewLoader()
	h := NewRulesHandler(nil, loader, "", "")
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/prompt-injection/manifest", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	hooks, ok := result["hooks"].([]any)
	if !ok || len(hooks) != 0 {
		t.Fatalf("hooks: want empty list, got %v", result["hooks"])
	}
	stages, ok := result["stages"].([]any)
	if !ok || len(stages) != 0 {
		t.Fatalf("stages: want empty list, got %v", result["stages"])
	}
}

func TestRulesHandler_CompilePreview(t *testing.T) {
	loader := pack.NewLoader()
	h := NewRulesHandler(nil, loader, "", "")
	mux := h.Routes()

	req := httptest.NewRequest("GET", "/api/prompt-injection/preview?breed=bianmu", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["breed_id"] != "bianmu" {
		t.Errorf("breed_id = %v, want bianmu", result["breed_id"])
	}
}
