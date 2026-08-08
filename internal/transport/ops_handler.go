package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"sounds-great-ai/internal/telemetry"
)

// OpsHandler exposes /api/ops/* telemetry endpoints.
type OpsHandler struct{}

func NewOpsHandler() *OpsHandler {
	return &OpsHandler{}
}

// RegisterRoutes registers the telemetry ops routes on the given mux.
func (h *OpsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ops/metrics", h.handleMetrics)
	mux.HandleFunc("/api/ops/metrics/history", h.handleMetricsHistory)
	mux.HandleFunc("/api/ops/traces", h.handleTraces)
}

func setOpsCors(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// GET /api/ops/metrics — Prometheus text format (latest snapshot)
func (h *OpsHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if setOpsCors(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	if ss := telemetry.SnapshotStoreInstance(); ss != nil {
		hist := ss.History(time.Time{})
		if len(hist) > 0 {
			w.Write([]byte(hist[len(hist)-1].Text))
			return
		}
	}
	// Fallback: collect directly from Prometheus handler
	if handler := telemetry.PromHandler(); handler != nil {
		handler.ServeHTTP(w, r)
		return
	}
	w.Write([]byte("# no metrics yet\n"))
}

// GET /api/ops/metrics/history?since=ISO8601
func (h *OpsHandler) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if setOpsCors(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ss := telemetry.SnapshotStoreInstance()
	if ss == nil {
		json.NewEncoder(w).Encode([]telemetry.MetricsSnapshot{})
		return
	}
	var since time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	json.NewEncoder(w).Encode(ss.History(since))
}

// GET /api/ops/traces?traceId=&breedId=&limit=
func (h *OpsHandler) handleTraces(w http.ResponseWriter, r *http.Request) {
	if setOpsCors(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ts := telemetry.TraceStoreInstance()
	if ts == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"spans": []interface{}{}, "stats": nil})
		return
	}
	traceID := r.URL.Query().Get("traceId")
	breedID := r.URL.Query().Get("breedId")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	spans := ts.Query(traceID, breedID, limit)
	stats := ts.Stats()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"spans": spans,
		"stats": stats,
	})
}
