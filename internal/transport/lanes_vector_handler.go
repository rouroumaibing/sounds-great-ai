package transport

import (
	"encoding/json"
	"net/http"

	"sounds-great-ai/internal/memory"
)

// SemanticSearch performs dense-vector recall over approved truth (Gap3). It is
// opt-in: when the embedder is unconfigured
// (SG_EMBED_API_KEY unset) it returns a clear 501 so the platform stays
// deterministic and lexical FTS5 search remains available.
func (h *LanesHandler) SemanticSearch(w http.ResponseWriter, r *http.Request) {
	if h.embedder == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "semantic search not configured (set SG_EMBED_API_KEY)",
		})
		return
	}
	var body struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	query := body.Query
	if query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}
	op := h.requestOperator(r)
	vec, err := h.embedder.Embed(r.Context(), query)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	topK := body.TopK
	if topK <= 0 {
		topK = 10
	}
	// Hybrid RRF (entry NN + passage NN + BM25 lexical), embedMode-gated.
	entries, _ := h.registry.HybridSearch(query, vec, topK, op)
	if entries == nil {
		entries = []*memory.LaneEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}

// Reindex embeds all approved truth so it becomes semantically searchable
// (Gap3). Idempotent: re-running overwrites existing vectors. Only meaningful
// when an embedder is configured; returns 501 otherwise.
func (h *LanesHandler) Reindex(w http.ResponseWriter, r *http.Request) {
	if h.embedder == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "semantic search not configured (set SG_EMBED_API_KEY)",
		})
		return
	}
	count := 0
	for _, t := range h.registry.LaneTypes() {
		for _, e := range h.registry.Lane(t).Truth() {
			vec, err := h.embedder.Embed(r.Context(), e.Content)
			if err != nil {
				continue
			}
			if h.registry.StoreVector(e.ID, vec) == nil {
				count++
			}
			// Passage-level vectors for sub-section recall (P1).
			if h.registry.StorePassages(e.ID, e.Content, func(p string) ([]float32, error) {
				return h.embedder.Embed(r.Context(), p)
			}) == nil {
				count++
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"indexed": count})
}
