package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sounds-great-ai/internal/mcp"
	"sounds-great-ai/internal/mcp/governance"
)

func TestMCPFallbackPlatform(t *testing.T) {
	store := mcp.NewFileStore(t.TempDir(), mcp.NewRegistry())
	store.SeedPlatform("/bin/platform-mcp", []string{"--api-base", "http://localhost:8080"}, nil, nil, "http://localhost:8080")
	probe := mcp.NewProbeCache(0, 0)
	h := NewMCPHandler(store, probe)

	req := httptest.NewRequest(http.MethodGet, "/api/mcp/servers/platform/fallback", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var view mcpFallbackView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if view.Name != "platform" {
		t.Fatalf("name = %q", view.Name)
	}
	if len(view.Tools) != len(governance.Catalog()) {
		t.Fatalf("expected %d fallback tools, got %d", len(governance.Catalog()), len(view.Tools))
	}
	if view.CallbackURL == "" {
		t.Fatal("expected callback_url")
	}
}

func TestMCPFallbackUnknown(t *testing.T) {
	store := mcp.NewFileStore(t.TempDir(), mcp.NewRegistry())
	h := NewMCPHandler(store, mcp.NewProbeCache(0, 0))
	req := httptest.NewRequest(http.MethodGet, "/api/mcp/servers/nope/fallback", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestMCPViewRemoteFields(t *testing.T) {
	store := mcp.NewFileStore(t.TempDir(), mcp.NewRegistry())
	if err := store.Add(mcp.MCPServerConfig{Name: "remote", URL: "https://mcp.example.com", Headers: map[string]string{"Authorization": "Bearer x"}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	probe := mcp.NewProbeCache(0, 0)
	h := NewMCPHandler(store, probe)
	req := httptest.NewRequest(http.MethodGet, "/api/mcp/servers", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var views []mcpServerView
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	v := views[0]
	if v.URL != "https://mcp.example.com" {
		t.Fatalf("url not in view: %+v", v)
	}
	if v.FallbackAvailable {
		t.Fatal("remote server without callback_url should not mark fallback_available")
	}
	if v.Headers["Authorization"] != "***" {
		t.Fatalf("header should be masked: %+v", v.Headers)
	}
}
