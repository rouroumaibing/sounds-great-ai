package transport

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sounds-great-ai/internal/settings"
)

func newTestPanels(t *testing.T) *PanelsHandler {
	t.Helper()
	return NewPanelsHandler(settings.NewPanelConfigStore(t.TempDir()))
}

func panelJSON(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		rdr = bytes.NewReader(data)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestPanelsHandler_ConciergeDefaultsAndPersistence(t *testing.T) {
	h := newTestPanels(t)
	mux := h.Routes()

	// Fresh store → documented defaults.
	rec := panelJSON(t, mux, "GET", "/api/config/concierge", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	var cfg map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg["avatar"] != "🐕" || cfg["proactivityLevel"] != "medium" {
		t.Errorf("defaults wrong: %v", cfg)
	}

	// PATCH validates and persists; a new handler over the same dir reads it back.
	rec = panelJSON(t, mux, "PATCH", "/api/config/concierge", map[string]any{
		"avatar":           "🐩",
		"color":            "#AA66CC",
		"greeting":         "汪！今天跑点什么？",
		"proactivityLevel": "high",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d body=%s, want 200", rec.Code, rec.Body.String())
	}
	cfg = map[string]any{}
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if cfg["avatar"] != "🐩" || cfg["proactivityLevel"] != "high" || cfg["color"] != "#AA66CC" {
		t.Errorf("patched config wrong: %v", cfg)
	}

	h2 := NewPanelsHandler(h.store)
	rec = panelJSON(t, h2.Routes(), "GET", "/api/config/concierge", nil)
	cfg = map[string]any{}
	_ = json.NewDecoder(rec.Body).Decode(&cfg)
	if cfg["greeting"] != "汪！今天跑点什么？" {
		t.Errorf("persistence lost: %v", cfg)
	}

	// Unset fields keep their value (partial update semantics).
	rec = panelJSON(t, mux, "PATCH", "/api/config/concierge", map[string]any{"avatar": "🐕"})
	if rec.Code != http.StatusOK {
		t.Fatalf("partial PATCH status = %d", rec.Code)
	}
	cfg = map[string]any{}
	_ = json.NewDecoder(rec.Body).Decode(&cfg)
	if cfg["proactivityLevel"] != "high" {
		t.Errorf("partial update clobbered proactivityLevel: %v", cfg)
	}
}

func TestPanelsHandler_ConciergeValidation(t *testing.T) {
	h := newTestPanels(t)
	mux := h.Routes()

	cases := []map[string]any{
		{"color": "red"}, // not #RRGGBB
		{"size": 8},      // below 16
		{"size": 999},    // above 256
		{"proactivityLevel": "ultra"},
		{"autoSuggestThreshold": 42},
	}
	for _, body := range cases {
		rec := panelJSON(t, mux, "PATCH", "/api/config/concierge", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PATCH %v status = %d, want 400", body, rec.Code)
		}
	}
}

func TestPanelsHandler_VoicePersistenceAndValidation(t *testing.T) {
	h := newTestPanels(t)
	mux := h.Routes()

	rec := panelJSON(t, mux, "GET", "/api/config/voice", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	var cfg map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&cfg)
	if cfg["ttsVoice"] != "alloy" || cfg["sttModel"] != "whisper-1" {
		t.Errorf("voice defaults wrong: %v", cfg)
	}

	rec = panelJSON(t, mux, "PATCH", "/api/config/voice", map[string]any{
		"enabled":  true,
		"ttsVoice": "zh-CN-Yunxi",
		"ttsSpeed": 1.25,
		"glossary": []map[string]string{{"source": "SG", "target": "Sounds Great"}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d body=%s", rec.Code, rec.Body.String())
	}
	cfg = map[string]any{}
	_ = json.NewDecoder(rec.Body).Decode(&cfg)
	if cfg["ttsVoice"] != "zh-CN-Yunxi" || cfg["ttsSpeed"] != 1.25 {
		t.Errorf("patched voice wrong: %v", cfg)
	}

	// Out-of-range speed rejected, config unchanged.
	rec = panelJSON(t, mux, "PATCH", "/api/config/voice", map[string]any{"ttsSpeed": 9.9})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("speed 9.9 status = %d, want 400", rec.Code)
	}
	rec = panelJSON(t, mux, "GET", "/api/config/voice", nil)
	cfg = map[string]any{}
	_ = json.NewDecoder(rec.Body).Decode(&cfg)
	if cfg["ttsSpeed"] != 1.25 {
		t.Errorf("rejected patch mutated state: %v", cfg)
	}
}

func TestPanelsHandler_ConnectorsCRUDAndMasking(t *testing.T) {
	h := newTestPanels(t)
	mux := h.Routes()

	// Empty registry on fresh store.
	rec := panelJSON(t, mux, "GET", "/api/config/connectors", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec.Code)
	}
	var list []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 0 {
		t.Errorf("fresh registry not empty: %v", list)
	}

	// Create with a secret; the response must mask it.
	rec = panelJSON(t, mux, "POST", "/api/config/connectors", map[string]any{
		"name":     "ops-webhook",
		"type":     "webhook",
		"endpoint": "https://example.com/hook",
		"auth_key": "secret-token-123",
		"enabled":  true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&created)
	if created["auth_key_set"] != true {
		t.Errorf("auth_key_set = %v, want true", created["auth_key_set"])
	}
	if s, _ := created["auth_key_preview"].(string); s == "secret-token-123" || s == "" {
		t.Errorf("auth_key_preview leaks or empty: %q", s)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("no connector id returned")
	}

	// Invalid creates are rejected.
	for _, body := range []map[string]any{
		{"name": "", "type": "webhook", "endpoint": "https://x.io"}, // empty name
		{"name": "x", "type": "icq", "endpoint": "https://x.io"},    // bad type
		{"name": "x", "type": "webhook", "endpoint": "ftp://x.io"},  // bad scheme
		{"name": "x", "type": "webhook", "endpoint": "not a url"},   // unparseable
	} {
		rec = panelJSON(t, mux, "POST", "/api/config/connectors", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("POST %v status = %d, want 400", body, rec.Code)
		}
	}

	// PATCH toggles enabled without touching the stored secret (nil auth_key).
	rec = panelJSON(t, mux, "PATCH", "/api/config/connectors/"+id, map[string]any{"enabled": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d", rec.Code)
	}
	var updated map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&updated)
	if updated["enabled"] != false || updated["auth_key_set"] != true {
		t.Errorf("PATCH lost state: %v", updated)
	}

	// GET still returns exactly one masked connector.
	rec = panelJSON(t, mux, "GET", "/api/config/connectors", nil)
	list = nil
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 || list[0]["auth_key_set"] != true {
		t.Errorf("GET after patch wrong: %v", list)
	}

	// DELETE removes it; a second DELETE 404s.
	rec = panelJSON(t, mux, "DELETE", "/api/config/connectors/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d", rec.Code)
	}
	rec = panelJSON(t, mux, "DELETE", "/api/config/connectors/"+id, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("second DELETE status = %d, want 404", rec.Code)
	}
}

func TestPanelsHandler_ConnectorProbe(t *testing.T) {
	// A local test server plays the connector endpoint.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer probe-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newTestPanels(t)
	mux := h.Routes()

	rec := panelJSON(t, mux, "POST", "/api/config/connectors", map[string]any{
		"name": "probe-target", "type": "generic", "endpoint": srv.URL, "auth_key": "probe-key",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d", rec.Code)
	}
	var created map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&created)
	id := created["id"].(string)

	// Probe with the right key: reachable.
	rec = panelJSON(t, mux, "POST", "/api/config/connectors/"+id+"/test", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe status = %d", rec.Code)
	}
	var result map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result["ok"] != true {
		t.Errorf("probe result = %v, want ok", result)
	}
	// The last_check summary is persisted on the connector.
	rec = panelJSON(t, mux, "GET", "/api/config/connectors", nil)
	var list []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if list[0]["last_check"] == "" {
		t.Errorf("last_check not persisted: %v", list[0])
	}

	// Wrong key → 401 surfaces as ok=false (auth failure is a probe result,
	// not a transport error).
	rec = panelJSON(t, mux, "PATCH", "/api/config/connectors/"+id, map[string]any{"auth_key": "wrong"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d", rec.Code)
	}
	rec = panelJSON(t, mux, "POST", "/api/config/connectors/"+id+"/test", nil)
	result = nil
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result["ok"] != false {
		t.Errorf("probe with wrong key = %v, want ok=false", result)
	}

	// Unreachable host → ok=false with error.
	rec = panelJSON(t, mux, "POST", "/api/config/connectors", map[string]any{
		"name": "dead", "type": "webhook", "endpoint": "http://127.0.0.1:1/nope",
	})
	var dead map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&dead)
	rec = panelJSON(t, mux, "POST", "/api/config/connectors/"+dead["id"].(string)+"/test", nil)
	result = nil
	_ = json.NewDecoder(rec.Body).Decode(&result)
	if result["ok"] != false || result["error"] == nil {
		t.Errorf("dead probe = %v, want ok=false + error", result)
	}
}

// Security regression: the handler must not set its own permissive CORS
// header — the central CORSMiddleware owns CORS policy.
func TestPanelsHandler_NoPermissiveCORSOverride(t *testing.T) {
	h := newTestPanels(t)
	rec := panelJSON(t, h.Routes(), "GET", "/api/config/concierge", nil)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Errorf("handler set Access-Control-Allow-Origin: * — central middleware must own CORS")
	}
}
