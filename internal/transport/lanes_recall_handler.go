package transport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/memory"
)

// matchesSensitivity reports whether an entry passes an optional sensitivity
// filter ("" = any).
func matchesSensitivity(e *memory.LaneEntry, sensitivity string) bool {
	if sensitivity == "" {
		return true
	}
	return e.Sensitivity == sensitivity
}

// RecallEvents returns recent memory-recall observations (injection
// observability). ?limit= caps the count.
func (h *LanesHandler) RecallEvents(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if h.recall == nil {
		respondJSON(w, http.StatusOK, []any{})
		return
	}
	respondJSON(w, http.StatusOK, h.recall.Recent(limit))
}

// RecallLedger returns recall counts within 7/14/30-day windows.
// ?windows=7,14,30 overrides the defaults.
func (h *LanesHandler) RecallLedger(w http.ResponseWriter, r *http.Request) {
	windows := []int{7, 14, 30}
	if v := strings.TrimSpace(r.URL.Query().Get("windows")); v != "" {
		var parsed []int
		for _, p := range strings.Split(v, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
				parsed = append(parsed, n)
			}
		}
		if len(parsed) > 0 {
			windows = parsed
		}
	}
	if h.recall == nil {
		res := make(map[string]memory.RecallWindowStat)
		for _, w := range windows {
			res[strconv.Itoa(w)+"d"] = memory.RecallWindowStat{}
		}
		respondJSON(w, http.StatusOK, res)
		return
	}
	respondJSON(w, http.StatusOK, h.recall.Ledger(windows))
}

// MarkOutcome records the consumption outcome (used/ignored) of a recall event,
// completing the consumption-verification loop (P0-1).
func (h *LanesHandler) MarkOutcome(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Outcome  string `json:"outcome"`
		Axis     string `json:"axis"`
		Maturity string `json:"maturity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Outcome != memory.RecallOutcomeUsed && body.Outcome != memory.RecallOutcomeIgnored) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "outcome must be 'used' or 'ignored'"})
		return
	}
	if h.recall == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "recall store unavailable"})
		return
	}
	op := h.explicitOperator(r)
	if err := h.recall.MarkOutcome(id, body.Outcome, body.Axis, body.Maturity, op); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PullRecall surfaces approved truth on demand (the "pull" half of the
// push/pull duality, P2-7). With ?query= it returns approved entries
// matching the query (FTS5); without it returns all approved truth visible to
// the operator. Records a pull recall event for observability.
func (h *LanesHandler) PullRecall(w http.ResponseWriter, r *http.Request) {
	op := h.requestOperator(r)
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	var entries []*memory.LaneEntry
	if query != "" {
		entries = h.registry.Search(query, op)
		// Surface only approved (truth) matches in a pull recall.
		approved := make([]*memory.LaneEntry, 0, len(entries))
		for _, e := range entries {
			if e.Status == memory.StatusApproved {
				approved = append(approved, e)
			}
		}
		entries = approved
	} else {
		entries, _ = h.registry.RecallEntries(50, op)
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	if h.recall != nil {
		h.recall.Record(&memory.RecallEvent{
			OperatorID: op,
			Kind:       "pull",
			Trigger:    "manual",
			EntryIDs:   ids,
			Count:      len(ids),
		})
		// Consumption lifecycle trace (P1 three-axis / Task #39).
		for _, e := range entries {
			h.recall.RecordLifecycle(memory.AxisConsumption, e.ID, string(e.Type), "pull_recall", memory.MaturityMeasured)
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"count":     len(entries),
		"entry_ids": ids,
		"entries":   entries,
	})
}

// Lifecycle returns the append-only lifecycle-trace records (P1 three-axis /
// Task #39). ?limit= caps the count.
func (h *LanesHandler) Lifecycle(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if h.recall == nil {
		respondJSON(w, http.StatusOK, []any{})
		return
	}
	respondJSON(w, http.StatusOK, h.recall.RecentLifecycle(limit))
}

// Search returns lane entries whose content matches the query (FTS5, P1-5).
func (h *LanesHandler) Search(w http.ResponseWriter, r *http.Request) {
	op := h.operatorFilter(r)
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "q required"})
		return
	}
	respondJSON(w, http.StatusOK, h.registry.Search(query, op))
}

// Reflect synthesizes an abstractive reflection over approved truth (P2-6). It
// is a SANCTIONED platform synthesis
// service (不可逆决策 §4.8), never agent reasoning. The output is
// returned to the caller and does NOT auto-become truth; with ?seed=true it is
// submitted as a PENDING candidate that still requires human disposition (M5).
// When the reflector is unconfigured (SG_REFLECT_* env unset) it returns a
// clear 501 so the platform stays deterministic by default.
func (h *LanesHandler) Reflect(w http.ResponseWriter, r *http.Request) {
	if h.reflector == nil {
		respondJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "reflection not configured (set SG_REFLECT_API_KEY + SG_REFLECT_MODEL, or SG_REFLECT_CLI)",
		})
		return
	}
	var body struct {
		Lane     string `json:"lane"`
		Focus    string `json:"focus"`
		Seed     bool   `json:"seed"`
		MaxChars int    `json:"max_chars"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	op := h.operatorFilter(r)
	entries, ok := h.registry.RecallEntries(100, op)
	if !ok {
		respondJSON(w, http.StatusOK, map[string]any{"reflection": "", "count": 0})
		return
	}
	if body.Lane != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if string(e.Type) == body.Lane {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}
	if len(entries) == 0 {
		respondJSON(w, http.StatusOK, map[string]any{"reflection": "", "count": 0})
		return
	}

	reflection, err := h.reflector.Reflect(r.Context(), entries, capability.ReflectOptions{
		MaxChars: body.MaxChars,
		Focus:    body.Focus,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := map[string]any{"reflection": reflection, "count": len(entries)}
	if body.Seed {
		seedLane := memory.LaneDecision
		if body.Lane != "" {
			if l := h.registry.Lane(memory.LaneType(body.Lane)); l != nil {
				seedLane = memory.LaneType(body.Lane)
			}
		}
		ids := memory.NewDeltaProducer().SubmitCandidates(h.registry, []memory.DeltaCandidate{{
			Lane:    seedLane,
			Content: reflection,
			Source:  "reflection:" + op,
		}}, op)
		resp["seeded_ids"] = ids
	}
	respondJSON(w, http.StatusOK, resp)
}
