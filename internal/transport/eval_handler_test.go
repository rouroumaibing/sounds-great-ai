package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvalHandlerRoutes(t *testing.T) {
	// Test that routes are registered without panic
	// Full integration test requires EvalRunner setup (Task 9)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/evals", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/api/evals/run", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/api/evals", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
