package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"sounds-great-ai/internal/memory"
)

// Link creates a typed relationship edge from one entry to another (Gap1).
// Both entry IDs must already exist in the registry. Edge-level sensitivity +
// provenance are accepted.
func (h *LanesHandler) Link(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		To             string `json:"to"`
		Relation       string `json:"relation"`
		EdgeSensitivity string `json:"edge_sensitivity"`
		Provenance     string `json:"provenance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.To == "" || body.Relation == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "to and relation required"})
		return
	}
	rel := memory.LaneRelation(body.Relation)
	if !memory.ValidRelation(rel) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown relation: " + body.Relation})
		return
	}
	op := h.requestOperator(r)
	edge, err := h.registry.AddEdgeFull(id, body.To, rel, body.EdgeSensitivity, body.Provenance, op)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, edge)
}

// Mark attaches a normalized signal (marker) to an entry (Gap1: captured →
// normalized → approved/rejected). The marker records *why* an entry matters
// without promoting it to a full lane entry.
func (h *LanesHandler) Mark(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		MarkerType string `json:"marker_type"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MarkerType == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "marker_type required"})
		return
	}
	op := h.requestOperator(r)
	m, err := h.registry.AddMarker(id, body.MarkerType, body.Content, op)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, m)
}

// Graph returns the outgoing edges and markers around an entry (Gap1), so the
// frontend can render the relationship graph. Returns empty slices when the
// graph store is unavailable (fail-open).
func (h *LanesHandler) Graph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	edges, markers := h.registry.Graph(id)
	if edges == nil {
		edges = []*memory.LaneEdge{}
	}
	if markers == nil {
		markers = []*memory.LaneMarker{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"edges":   edges,
		"markers": markers,
	})
}

// CueEvents returns recent cue-consumption ledger events (Gap4). Surfaced so
// the operator can audit which approved truth was injected and at what
// rank/score.
func (h *LanesHandler) CueEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	respondJSON(w, http.StatusOK, h.registry.CueEvents(limit))
}

// SetSensitivity re-classifies an entry's sensitivity level (Gap2). The level
// must be one of public/internal/private/restricted; the new level is enforced
// on subsequent recall reads via EntryVisible (clearance + collection grant).
//
// Visibility-widening guardrail (Task #33): relaxing an entry's sensitivity to
// a *wider* level (e.g. private/restricted → internal/public) requires an
// explicit confirm_visibility_widening=true, otherwise the change is rejected
// with a 409 so a caller must consciously widen visibility.
func (h *LanesHandler) SetSensitivity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Sensitivity              string `json:"sensitivity"`
		ConfirmVisibilityWidening bool  `json:"confirm_visibility_widening"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !memory.ValidSensitivity(body.Sensitivity) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "valid sensitivity required (public/internal/private/restricted)"})
		return
	}
	oldSens, ok := h.registry.SensitivityOf(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
		return
	}
	// Widening = new rank is LOWER (wider) than old rank.
	if memory.SensitivityRank(body.Sensitivity) < memory.SensitivityRank(oldSens) && !body.ConfirmVisibilityWidening {
		respondJSON(w, http.StatusConflict, map[string]string{
			"error":            "visibility widening requires confirmation",
			"current":         oldSens,
			"requested":       body.Sensitivity,
			"confirm_field":   "confirm_visibility_widening",
		})
		return
	}
	if !h.registry.SetSensitivity(id, body.Sensitivity) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found or invalid sensitivity"})
		return
	}
	// Audit the access-posture reclassification under the acting operator
	// (multi-operator attribution; no dedicated column, so it rides the
	// append-only lifecycle trace detail).
	if op := h.explicitOperator(r); h.recall != nil {
		detail := "sensitivity: " + oldSens + "→" + body.Sensitivity
		if op != "" {
			detail += " (op: " + op + ")"
		}
		h.recall.RecordLifecycle(memory.AxisCorrection, id, "", detail, memory.MaturityMeasured)
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
