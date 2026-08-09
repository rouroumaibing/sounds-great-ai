package http

import (
	"encoding/json"
	"net/http"
	"runtime"

	"sounds-great-ai/internal/telemetry"
)

// SkillsHandler returns a handler that lists available skills.
func SkillsHandler(skillsList func() []SkillItem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		items := skillsList()
		if items == nil {
			items = []SkillItem{}
		}
		json.NewEncoder(w).Encode(items)
	}
}

// SkillItem represents a skill in the API response.
type SkillItem struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// MCPServersHandler returns a handler that lists registered MCP servers.
func MCPServersHandler(serversList func() []MCPItem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		items := serversList()
		if items == nil {
			items = []MCPItem{}
		}
		json.NewEncoder(w).Encode(items)
	}
}

// MCPItem represents an MCP server in the API response.
type MCPItem struct {
	Name    string   `json:"name"`
	Tools   []string `json:"tools"`
	Enabled bool     `json:"enabled"`
}

// HealthHandler returns a handler for the /api/ops/health endpoint.
func HealthHandler(uptimeStr func() string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		otelStatus := "disabled"
		if telemetry.IsInitialized() {
			otelStatus = "ok"
		}

		json.NewEncoder(w).Encode(map[string]any{
			"uptime":           uptimeStr(),
			"goroutines":       runtime.NumGoroutine(),
			"mem_alloc":        mem.Alloc,
			"mem_total_alloc":  mem.TotalAlloc,
			"mem_sys":          mem.Sys,
			"mem_heap_alloc":   mem.HeapAlloc,
			"mem_heap_sys":     mem.HeapSys,
			"mem_heap_objects": mem.HeapObjects,
			"mem_num_gc":       mem.NumGC,
			"status":           "ok",
			"otel": map[string]any{
				"status":         otelStatus,
				"tracesEnabled":  telemetry.IsInitialized(),
				"metricsEnabled": telemetry.IsInitialized(),
			},
		})
	}
}

// ReadyHandler returns a handler for the /ready endpoint.
func ReadyHandler(readyCheck func() (bool, int, int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, adapters, breeds := readyCheck()
		w.Header().Set("Content-Type", "application/json")
		if ok {
			json.NewEncoder(w).Encode(map[string]any{
				"status":   "ready",
				"adapters": adapters,
				"breeds":   breeds,
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
		}
	}
}

// SimpleHealthHandler returns a handler that always returns "ok".
func SimpleHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
