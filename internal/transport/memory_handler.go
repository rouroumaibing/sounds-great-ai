package transport

import (
	"encoding/json"
	"net/http"

	"sounds-great-ai/internal/memory"
)

// MemoryHandler handles memory evidence HTTP endpoints.
type MemoryHandler struct {
	store memory.EvidenceStore
}

// NewMemoryHandler creates a new MemoryHandler.
func NewMemoryHandler(store memory.EvidenceStore) *MemoryHandler {
	return &MemoryHandler{store: store}
}

// Routes returns the HTTP routes for memory.
func (h *MemoryHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/memory/evidence", h.ListEvidence)
	mux.HandleFunc("POST /api/memory/evidence", h.AddEvidence)
	return mux
}

func (h *MemoryHandler) ListEvidence(w http.ResponseWriter, r *http.Request) {
	evidence, err := h.store.ListEvidence()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, evidence)
}

func (h *MemoryHandler) AddEvidence(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content  string   `json:"content"`
		Type     string   `json:"type"`
		Title    string   `json:"title"`
		ThreadID string   `json:"thread_id"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	rec, err := h.store.AddEvidence(body.ThreadID, body.Type, body.Title, body.Content, body.Tags)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, rec)
}
