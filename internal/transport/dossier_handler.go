package transport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"sounds-great-ai/internal/dossier"
	"sounds-great-ai/pkg/pack"
)

// DossierHandler exposes the capability-dossier domain (FT-DS-001):
//
//	GET  /api/dossier                                — profiles × catalog join, grouped by model
//	GET  /api/dossier/base-hash                      — current file hash (for proposal creators)
//	POST/GET /api/dossier/observations               — operator observation staging layer
//	GET  /api/dossier/distillation-opportunities     — pending opportunities (scoped)
//	POST /api/dossier/distillation-opportunities/{id}/dismiss | /convert
//	POST/GET /api/dossier/distillations              — proposal create (idempotent) / list
//	GET  /api/dossier/distillations/{id}             — proposal detail
//	POST /api/dossier/distillations/{id}/approve | /reject | /execute-apply
//
// Governance mirrors the SG convention: the state machine is the guard.
// Actor identity comes from the `actor` body field or X-SG-Actor header
// (default "operator"); separation of duties (no self-approval) and
// target-dog-only apply are enforced in the service layer.
type DossierHandler struct {
	svc    *dossier.Service
	loader *dossier.Loader
	// breeds provides the current breed configs for the join endpoint.
	breeds func() map[string]*pack.BreedConfig
}

// NewDossierHandler creates the handler.
func NewDossierHandler(svc *dossier.Service, loader *dossier.Loader, breeds func() map[string]*pack.BreedConfig) *DossierHandler {
	return &DossierHandler{svc: svc, loader: loader, breeds: breeds}
}

// Routes returns the dossier HTTP routes.
func (h *DossierHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/dossier", h.handleDossierOverview)
	mux.HandleFunc("/api/dossier/base-hash", h.handleBaseHash)
	mux.HandleFunc("/api/dossier/observations", h.handleObservations)
	mux.HandleFunc("/api/dossier/distillation-opportunities", h.handleOpportunities)
	mux.HandleFunc("/api/dossier/distillation-opportunities/{id}/dismiss", h.handleOpportunityDismiss)
	mux.HandleFunc("/api/dossier/distillation-opportunities/{id}/convert", h.handleOpportunityConvert)
	mux.HandleFunc("/api/dossier/distillations", h.handleDistillations)
	mux.HandleFunc("/api/dossier/distillations/{id}", h.handleDistillationDetail)
	mux.HandleFunc("/api/dossier/distillations/{id}/approve", h.handleDistillationApprove)
	mux.HandleFunc("/api/dossier/distillations/{id}/reject", h.handleDistillationReject)
	mux.HandleFunc("/api/dossier/distillations/{id}/execute-apply", h.handleDistillationApply)
	return mux
}

// ---- Overview join (profiles × catalog, grouped by model) ----

type dossierDogCard struct {
	DogID       string                  `json:"dogId"`
	BreedID     string                  `json:"breedId"`
	DisplayName string                  `json:"displayName"`
	VariantID   string                  `json:"variantId,omitempty"`
	Channel     string                  `json:"channel,omitempty"` // client id (claude/codex/…)
	Dossier     *dossier.DossierProfile `json:"dossier"`
}

type dossierModelGroup struct {
	Model string           `json:"model"`
	Dogs  []dossierDogCard `json:"dogs"`
}

type dossierOverview struct {
	ModelGroups []dossierModelGroup `json:"modelGroups"`
	Meta        dossierOverviewMeta `json:"meta"`
}

type dossierOverviewMeta struct {
	TotalDogs        int     `json:"totalDogs"`
	TotalModels      int     `json:"totalModels"`
	DossierCoverage  float64 `json:"dossierCoverage"`
	DossierAvailable bool    `json:"dossierAvailable"`
}

func (h *DossierHandler) handleDossierOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	workspace := workspaceRoot(h.svc)
	profiles := h.loader.Load(workspace)

	modelGroups := map[string][]dossierDogCard{}
	totalDogs, covered := 0, 0

	if h.breeds != nil {
		for breedID, breed := range h.breeds() {
			if !breed.Enabled && breed.ID == "a2a-remote" {
				continue
			}
			for _, variant := range breed.Variants {
				dogID := variant.DogID
				if dogID == "" {
					dogID = breed.DogID
				}
				if dogID == "" {
					dogID = breedID
				}
				totalDogs++
				var profile *dossier.DossierProfile
				if p, ok := profiles[dogID]; ok {
					profile = &p
					covered++
				}
				model := variant.DefaultModel
				if model == "" {
					model = "unknown"
				}
				modelGroups[model] = append(modelGroups[model], dossierDogCard{
					DogID:       dogID,
					BreedID:     breedID,
					DisplayName: breed.DisplayName,
					VariantID:   variant.ID,
					Channel:     variant.ClientID,
					Dossier:     profile,
				})
			}
		}
	}

	groups := make([]dossierModelGroup, 0, len(modelGroups))
	for model, dogs := range modelGroups {
		groups = append(groups, dossierModelGroup{Model: model, Dogs: dogs})
	}

	coverage := 0.0
	if totalDogs > 0 {
		coverage = float64(covered) / float64(totalDogs)
	}
	respondJSON(w, http.StatusOK, dossierOverview{
		ModelGroups: groups,
		Meta: dossierOverviewMeta{
			TotalDogs:        totalDogs,
			TotalModels:      len(groups),
			DossierCoverage:  coverage,
			DossierAvailable: h.loader.IsAvailable(workspace),
		},
	})
}

func (h *DossierHandler) handleBaseHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	hash, err := h.svc.CurrentBaseHash()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"baseHash": hash})
}

// ---- Observations ----

func (h *DossierHandler) handleObservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			DogID   string `json:"dogId"`
			Content string `json:"content"`
			Actor   string `json:"actor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if strings.TrimSpace(body.DogID) == "" || strings.TrimSpace(body.Content) == "" {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "dogId and content are required"})
			return
		}
		obs, err := h.svc.Observations.Add(dossier.AddObservationInput{
			DogID:   body.DogID,
			Content: strings.TrimSpace(body.Content),
			Author:  actorOf(r, body.Actor),
		})
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusCreated, map[string]dossier.Observation{"observation": obs})
	case http.MethodGet:
		q := r.URL.Query()
		limit := parseLimit(q.Get("limit"))
		if dogID := q.Get("dogId"); dogID != "" {
			obs, err := h.svc.Observations.List(dogID, limit)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if obs == nil {
				obs = []dossier.Observation{}
			}
			respondJSON(w, http.StatusOK, map[string]any{"observations": obs})
			return
		}
		grouped, err := h.svc.Observations.ListAll(limit)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if grouped == nil {
			grouped = map[string][]dossier.Observation{}
		}
		respondJSON(w, http.StatusOK, map[string]any{"observations": grouped})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST"})
	}
}

// ---- Opportunities ----

func (h *DossierHandler) handleOpportunities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	all := h.svc.Opportunities.ListPending()
	// Scope: the operator sees everything; a dog actor sees only its own.
	if actor := actorOf(r, ""); actor != "" && actor != "operator" && actor != "leader" {
		scoped := make([]dossier.DistillationOpportunity, 0, len(all))
		for _, op := range all {
			if op.TargetDogID == actor {
				scoped = append(scoped, op)
			}
		}
		all = scoped
	}
	if all == nil {
		all = []dossier.DistillationOpportunity{}
	}
	respondJSON(w, http.StatusOK, map[string]any{"opportunities": all})
}

func (h *DossierHandler) handleOpportunityDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	id := r.PathValue("id")
	if !h.svc.Opportunities.Dismiss(id) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "opportunity not found or already processed"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *DossierHandler) handleOpportunityConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var body struct {
		ProposalID string `json:"proposalId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ProposalID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "proposalId is required"})
		return
	}
	if !h.svc.Opportunities.MarkConverted(r.PathValue("id"), body.ProposalID) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "opportunity not found or already processed"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- Distillation proposals ----

func (h *DossierHandler) handleDistillations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var wire struct {
			SourceEvent    string                `json:"sourceEvent"`
			SourceID       string                `json:"sourceId"`
			TargetDogID    string                `json:"targetDogId"`
			TargetFields   []string              `json:"targetFields"`
			BeforeSnapshot string                `json:"beforeSnapshot"`
			AfterDraft     string                `json:"afterDraft"`
			Rationale      string                `json:"rationale"`
			EvidenceRefs   []dossier.EvidenceRef `json:"evidenceRefs"`
			BaseHash       string                `json:"baseHash"`
			Actor          string                `json:"actor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&wire); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		proposal, created, err := h.svc.CreateProposal(dossier.CreateProposalInput{
			SourceEvent:    wire.SourceEvent,
			SourceID:       wire.SourceID,
			TargetDogID:    wire.TargetDogID,
			TargetFields:   wire.TargetFields,
			BeforeSnapshot: wire.BeforeSnapshot,
			AfterDraft:     wire.AfterDraft,
			Rationale:      wire.Rationale,
			EvidenceRefs:   wire.EvidenceRefs,
			BaseHash:       wire.BaseHash,
			CreatedBy:      actorOf(r, wire.Actor),
		})
		if err != nil {
			if errors.Is(err, dossier.ErrValidation) {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		status := http.StatusCreated
		if !created {
			status = http.StatusOK // idempotent hit
		}
		respondJSON(w, status, map[string]dossier.DistillationProposal{"proposal": proposal})
	case http.MethodGet:
		q := r.URL.Query()
		limit := parseLimit(q.Get("limit"))
		if dogID := q.Get("dogId"); dogID != "" {
			proposals, err := h.svc.Proposals.ListByDog(dogID, limit)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			respondJSON(w, http.StatusOK, map[string]any{"proposals": nonNilProposals(proposals)})
			return
		}
		proposals, err := h.svc.Proposals.ListPending(limit)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"proposals": nonNilProposals(proposals)})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST"})
	}
}

// nonNilProposals normalizes a nil slice to empty so JSON encodes [] not
// null (frontend type safety).
func nonNilProposals(p []dossier.DistillationProposal) []dossier.DistillationProposal {
	if p == nil {
		return []dossier.DistillationProposal{}
	}
	return p
}

func (h *DossierHandler) handleDistillationDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	proposal, err := h.svc.Proposals.Get(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "proposal not found"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]dossier.DistillationProposal{"proposal": proposal})
}

func (h *DossierHandler) handleDistillationApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var body struct {
		Actor string `json:"actor"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	proposal, err := h.svc.ApproveProposal(r.PathValue("id"), actorOf(r, body.Actor))
	if err != nil {
		respondDossierError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]dossier.DistillationProposal{"proposal": proposal})
}

func (h *DossierHandler) handleDistillationReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var body struct {
		Actor  string `json:"actor"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	proposal, err := h.svc.RejectProposal(r.PathValue("id"), actorOf(r, body.Actor), body.Reason)
	if err != nil {
		respondDossierError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]dossier.DistillationProposal{"proposal": proposal})
}

func (h *DossierHandler) handleDistillationApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var body struct {
		Actor string `json:"actor"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	result, err := h.svc.ExecuteApply(r.PathValue("id"), actorOf(r, body.Actor))
	if err != nil {
		respondDossierError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

// ---- helpers ----

func respondDossierError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dossier.ErrValidation):
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, dossier.ErrSeparationOfDuties):
		respondJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, dossier.ErrNotTargetDog):
		respondJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, dossier.ErrProposalNotFound):
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "proposal not found"})
	case errors.Is(err, dossier.ErrProposalState):
		respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		var applyErr *dossier.ApplyError
		if errors.As(err, &applyErr) {
			switch applyErr.Code {
			case dossier.ErrCodeBaseHashMismatch:
				respondJSON(w, http.StatusConflict, map[string]string{"error": applyErr.Message, "code": applyErr.Code, "currentHash": applyErr.CurrentHash})
			default:
				respondJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": applyErr.Message, "code": applyErr.Code})
			}
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

// actorOf resolves the acting identity: explicit body field > X-SG-Actor
// header > "operator". Browser flows default to the operator; dogs calling
// via the platform MCP pass their dogId as actor.
func actorOf(r *http.Request, bodyActor string) string {
	if bodyActor != "" {
		return bodyActor
	}
	if h := r.Header.Get("X-SG-Actor"); h != "" {
		return h
	}
	return "operator"
}

func parseLimit(s string) int {
	if s == "" {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 200 {
		return 100
	}
	return n
}

func workspaceRoot(svc *dossier.Service) string {
	return svc.WorkspaceDir
}
