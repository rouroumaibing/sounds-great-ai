package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sounds-great-ai/internal/ops"
	"sounds-great-ai/internal/sop"
	"sounds-great-ai/internal/transport"
	"sounds-great-ai/pkg/pack"
)

func TestHealthEndpoint(t *testing.T) {
	mux := BuildMux()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "ok" {
		t.Errorf("expected 'ok', got '%s'", string(body))
	}
}

func TestWebSocketEndpointExists(t *testing.T) {
	mux := BuildMux()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Error("expected /ws endpoint to exist")
	}
}

// TestQCAutoRunnerEndpoints verifies the server-side QC auto-trigger is wired:
// /api/qc/status returns the heartbeat snapshot and /api/qc/run triggers an
// on-demand pass. Auth is disabled (AUTH_TOKEN unset) so the handlers run.
func TestQCAutoRunnerEndpoints(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOUNDS_GREAT_AI_GLOBAL_CONFIG_ROOT", dir)
	runner := sop.NewAutoRunner(dir, true)
	p := pack.New("default")
	mux := BuildMuxWithHandler(transport.NewWSHandler(p), p, nil, nil, nil, dir, time.Now(), nil, ops.NewLogBuffer(1000), runner)

	req := httptest.NewRequest(http.MethodGet, "/api/qc/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/qc/status: expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/qc/run", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("POST /api/qc/run: expected 200, got %d (%s)", w2.Code, w2.Body.String())
	}

	// After a run, status should reflect a recorded pass.
	req3 := httptest.NewRequest(http.MethodGet, "/api/qc/status", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("GET /api/qc/status (after run): expected 200, got %d", w3.Code)
	}
}
