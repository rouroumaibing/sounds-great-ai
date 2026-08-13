package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sounds-great-ai/internal/eval"
)

// EvalStore is the storage port the EvalHandler depends on (G10 port
// abstraction). *eval.ResultStore satisfies it structurally, so production
// wiring is unchanged while tests can inject a mock store.
type EvalStore interface {
	ListVerdicts(domainID string) ([]eval.VerdictHandoffPacket, error)
	GetVerdict(verdictID string) (*eval.VerdictHandoffPacket, error)
}

// EvalHandler handles eval HTTP endpoints.
type EvalHandler struct {
	runner    *eval.EvalRunner
	store     EvalStore
	closure   eval.ClosureService
	scheduler *eval.Scheduler
}

// NewEvalHandler creates a new EvalHandler.
func NewEvalHandler(runner *eval.EvalRunner, store EvalStore, closure eval.ClosureService, scheduler *eval.Scheduler) *EvalHandler {
	return &EvalHandler{runner: runner, store: store, closure: closure, scheduler: scheduler}
}

// Routes returns the HTTP routes for eval.
func (h *EvalHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/evals", h.handleEvals)
	mux.HandleFunc("/api/evals/run", h.handleRun)
	mux.HandleFunc("/api/evals/results", h.handleResults)
	mux.HandleFunc("/api/evals/results/", h.handleResultDetail)
	return mux
}

func (h *EvalHandler) handleEvals(w http.ResponseWriter, r *http.Request) {
	setEvalCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	domains := h.runner.Domains()
	type domainSummary struct {
		Domain        eval.EvalDomain             `json:"domain"`
		LatestVerdict *eval.VerdictHandoffPacket  `json:"latestVerdict"`
	}
	summaries := make([]domainSummary, 0, len(domains))
	for _, d := range domains {
		verdicts, _ := h.store.ListVerdicts(d.DomainID)
		var latest *eval.VerdictHandoffPacket
		if len(verdicts) > 0 {
			latest = &verdicts[len(verdicts)-1]
		}
		summaries = append(summaries, domainSummary{Domain: d, LatestVerdict: latest})
	}
	respondJSON(w, http.StatusOK, summaries)
}

func (h *EvalHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	setEvalCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var body struct {
		DomainID string `json:"domainId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.scheduler.TriggerNow(context.Background(), body.DomainID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (h *EvalHandler) handleResults(w http.ResponseWriter, r *http.Request) {
	setEvalCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	domainID := r.URL.Query().Get("domainId")
	verdicts, err := h.store.ListVerdicts(domainID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, verdicts)
}

func (h *EvalHandler) handleResultDetail(w http.ResponseWriter, r *http.Request) {
	setEvalCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/evals/results/")
	parts := strings.SplitN(path, "/", 2)
	verdictID := parts[0]

	if len(parts) == 2 && parts[1] == "lifecycle" {
		h.handleLifecycle(w, r, verdictID)
		return
	}
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	v, err := h.store.GetVerdict(verdictID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, v)
}

func (h *EvalHandler) handleLifecycle(w http.ResponseWriter, r *http.Request, verdictID string) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var event eval.ClosureEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := h.closure.AppendEvent(context.Background(), verdictID, event); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	state, _ := h.closure.CurrentState(context.Background(), verdictID)
	respondJSON(w, http.StatusOK, map[string]string{"state": string(state)})
}

func setEvalCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
