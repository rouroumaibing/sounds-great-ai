package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceMiddleware_NotInitialized(t *testing.T) {
	handler := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestTraceMiddleware_Initialized(t *testing.T) {
	cleanup, _ := Init()
	defer cleanup()
	handler := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// SSE endpoints assert http.Flusher on the ResponseWriter they receive; the
// statusRecorder wrapper used to hide it, 500ing every streaming endpoint
// ("streaming unsupported") whenever telemetry was initialized.
func TestTraceMiddleware_PreservesFlusher(t *testing.T) {
	cleanup, _ := Init()
	defer cleanup()
	handler := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("ResponseWriter through TraceMiddleware does not implement http.Flusher")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
