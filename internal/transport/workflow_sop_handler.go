package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"sounds-great-ai/internal/sop"
)

// WorkflowSOPHandler exposes the SOP workflow bulletin board over REST.
//
// The board is information sharing, not flow control: agents read and write
// the stage, baton holder, next skill, resume capsule and check attestations
// for a feature so a handoff survives cold start and context compaction.
// All writes are CAS-guarded by the expected stage (409 on conflict) and
// transitions are validated against the rule-driven stage machine in
// internal/sop.WorkflowSOP — the board never hardcodes a DAG.
type WorkflowSOPHandler struct {
	store *sop.WorkflowSOP
}

// NewWorkflowSOPHandler creates a handler backed by the given store.
func NewWorkflowSOPHandler(store *sop.WorkflowSOP) *WorkflowSOPHandler {
	return &WorkflowSOPHandler{store: store}
}

// Routes registers the workflow-sop endpoints.
func (h *WorkflowSOPHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/backlog/{itemId}/workflow-sop", h.GetWorkflowSOP)
	mux.HandleFunc("PUT /api/backlog/{itemId}/workflow-sop", h.PutWorkflowSOP)
	return mux
}

// GetWorkflowSOP returns the current bulletin board for a feature.
// 404 when no board exists yet (agents create it with the first PUT).
func (h *WorkflowSOPHandler) GetWorkflowSOP(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")
	state, err := h.store.GetState(r.Context(), itemID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if state == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Workflow SOP not found"})
		return
	}
	respondJSON(w, http.StatusOK, state)
}

// workflowSOPUpdate is the PUT body. Every field is optional except the
// feature binding (see PutWorkflowSOP). Empty strings mean "leave unchanged";
// checks are upserted by name so a caller can attest a single check without
// rewriting the whole board.
type workflowSOPUpdate struct {
	FeatureID     string              `json:"feature_id"`
	Stage         string              `json:"stage"`
	BatonHolder   string              `json:"baton_holder"`
	NextSkill     string              `json:"next_skill"`
	ResumeCapsule string              `json:"resume_capsule"`
	Checks        []sop.WorkflowCheck `json:"checks"`
	ExpectedStage string              `json:"expected_stage"`
}

// PutWorkflowSOP creates or updates the bulletin board for a feature.
//
// Validation, in order:
//   - body must be valid JSON (400)
//   - feature_id must match the path item id (422 feature_mismatch)
//   - a new board must start at kickoff; a stage change must be a valid
//     transition per the rule-driven state machine (400)
//   - expected_stage must match the persisted stage, otherwise the write is
//     rejected (409 concurrent_modification) and the caller re-reads
func (h *WorkflowSOPHandler) PutWorkflowSOP(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("itemId")

	var upd workflowSOPUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if upd.FeatureID != "" && upd.FeatureID != itemID {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "feature_id does not match backlog item id",
			"code":  "feature_mismatch",
		})
		return
	}
	featureID := itemID
	if upd.FeatureID != "" {
		featureID = upd.FeatureID
	}

	state, err := h.store.GetState(r.Context(), featureID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if state == nil {
		if upd.Stage != "" && upd.Stage != "kickoff" {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "new workflow board must start at stage kickoff",
			})
			return
		}
		state = &sop.WorkflowState{FeatureID: featureID, Stage: upd.Stage}
	} else if upd.Stage != "" && upd.Stage != state.Stage {
		if !sop.IsValidTransition(state.Stage, upd.Stage) {
			respondJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid transition " + state.Stage + " -> " + upd.Stage,
			})
			return
		}
		state.Stage = upd.Stage
	}

	if upd.BatonHolder != "" {
		state.BatonHolder = upd.BatonHolder
	}
	if upd.NextSkill != "" {
		state.NextSkill = upd.NextSkill
	}
	if upd.ResumeCapsule != "" {
		state.ResumeCapsule = upd.ResumeCapsule
	}
	for _, c := range upd.Checks {
		if c.Name == "" {
			continue
		}
		state.Checks = upsertWorkflowCheck(state.Checks, c)
	}

	if err := h.store.SetState(r.Context(), *state, upd.ExpectedStage); err != nil {
		if errors.Is(err, sop.ErrConcurrentModification) {
			respondJSON(w, http.StatusConflict, map[string]string{
				"error": "concurrent modification detected; re-read and retry with the current stage",
				"code":  "concurrent_modification",
			})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, state)
}

// upsertWorkflowCheck inserts or replaces a check by name.
func upsertWorkflowCheck(checks []sop.WorkflowCheck, newCheck sop.WorkflowCheck) []sop.WorkflowCheck {
	for i, c := range checks {
		if c.Name == newCheck.Name {
			checks[i] = newCheck
			return checks
		}
	}
	return append(checks, newCheck)
}
