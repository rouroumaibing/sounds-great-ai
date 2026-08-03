package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/cloudwego/eino/components/embedding"
	"sounds-great-ai/internal/ragstore"
)

// RAGHandler exposes HTTP endpoints for the frontend to inspect the active RAG
// backend, switch backends, trigger data sync, and query sync progress.
type RAGHandler struct {
	registry     *ragstore.StoreRegistry
	embedder     embedding.Embedder
	workspaceDir string
}

// NewRAGHandler builds a RAGHandler bound to the given registry, embedder, and
// workspace directory. The embedder is injected into StoreConfig on Switch so
// NewStore can build a functional backend (NewStore rejects a nil embedder).
// The workspaceDir is used to derive PersistPath/SQLitePath on Switch so
// HTTP-switched backends persist to the same workspace-relative files as the
// initial backend built by setupRAG.
func NewRAGHandler(registry *ragstore.StoreRegistry, embedder embedding.Embedder, workspaceDir string) *RAGHandler {
	return &RAGHandler{registry: registry, embedder: embedder, workspaceDir: workspaceDir}
}

// Routes returns a ServeMux with the four RAG API endpoints mounted.
func (h *RAGHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rag/backend", h.handleBackend)
	mux.HandleFunc("/api/rag/backend/switch", h.handleSwitch)
	mux.HandleFunc("/api/rag/sync", h.handleSync)
	mux.HandleFunc("/api/rag/sync/progress", h.handleSyncProgress)
	return mux
}

func (h *RAGHandler) handleBackend(w http.ResponseWriter, r *http.Request) {
	_, bk := h.registry.Active()
	retirees := h.registry.Retirees()
	retireeList := make([]map[string]any, 0, len(retirees))
	for _, ri := range retirees {
		retireeList = append(retireeList, map[string]any{
			"backend":    string(ri.Backend),
			"retired_at": ri.RetiredAt,
			"retire_at":  ri.RetireAt,
		})
	}
	json.NewEncoder(w).Encode(map[string]any{
		"active":   string(bk),
		"retirees": retireeList,
	})
}

type switchRequest struct {
	Backend string `json:"backend"`
}

func (h *RAGHandler) handleSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req switchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	cfg := ragstore.StoreConfig{
		Backend:     ragstore.BackendType(req.Backend),
		Embedder:    h.embedder,
		PersistPath: filepath.Join(h.workspaceDir, "rag_index.json"),
		SQLitePath:  filepath.Join(h.workspaceDir, "rag_index.db"),
	}
	if err := h.registry.Switch(context.Background(), ragstore.BackendType(req.Backend), cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "switched"})
}

func (h *RAGHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	m := h.registry.Migrator()
	if m == nil {
		http.Error(w, "migrator not initialized", http.StatusInternalServerError)
		return
	}
	var req struct {
		From string `json:"from"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	progress, err := m.SyncData(context.Background(), ragstore.BackendType(req.From))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(progress)
}

func (h *RAGHandler) handleSyncProgress(w http.ResponseWriter, r *http.Request) {
	m := h.registry.Migrator()
	if m == nil {
		http.Error(w, "migrator not initialized", http.StatusInternalServerError)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	progress, err := m.QueryProgress(r.Context(), ragstore.BackendType(from), ragstore.BackendType(to))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(progress)
}
