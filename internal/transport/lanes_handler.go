package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"sounds-great-ai/internal/capability"
	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/telemetry"
)

// LanesHandler exposes the typed-lane Shared Memory registry over HTTP.
//
// Human disposition (approve/reject/modify/retire/forget/defer/undo) is the
// ONLY way a pending candidate becomes canonical truth (M5 提交权): the
// platform stores and projects only what an operator explicitly approves. No
// reasoning runs inside the platform (不可逆决策
// §4.1). The DeltaProducer that creates candidates is deterministic pattern
// matching — no LLM (VISION §3).
//
// Multi-operator partitioning: entries carry operator_id; listing endpoints
// accept ?operator= to scope to one operator's truth plus shared ("")
// entries (KD-1). Recall events are
// recorded by the server at injection time and exposed for the frontend
// RecallFeed / RecallLedger.
//
// LLM reflection (MemoryReflector) is a SANCTIONED platform synthesis service
// under 不可逆决策 §4.8 (and §4.4 "平台合成走 Eino"). It is NOT agent
// reasoning: it only summarizes
// already-approved truth, and its output never auto-becomes truth (M5 提交权).
// It is opt-in — reflector is nil when SG_REFLECT_* env is unset, and the
// Reflect endpoint then returns a clear "not configured" error.
type LanesHandler struct {
	registry        *memory.LaneRegistry
	dispositions    *memory.DispositionRecorder
	recall          *memory.RecallStore
	reflector       MemoryReflector
	embedder        LaneEmbedder
	defaultOperator string
}

// MemoryReflector is the LLM abstractive-summary boundary for Shared Memory.
// It is intentionally an interface so the handler stays decoupled from the
// capability package internals and is trivially testable.
type MemoryReflector interface {
	Reflect(ctx context.Context, entries []*memory.LaneEntry, opts capability.ReflectOptions) (string, error)
}

// LaneEmbedder is the dense-vector embedder for semantic recall (Gap3). It is
// an interface so the handler stays decoupled and is testable with a stub.
type LaneEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SetEmbedder installs the optional semantic-search embedder (opt-in via env).
// When nil, semantic search returns 501.
func (h *LanesHandler) SetEmbedder(e LaneEmbedder) { h.embedder = e }

// NewLanesHandler creates the handler. defaultOperator is the leader name (used
// as the "by" identity on recorded dispositions when no finer identity exists).
// reflector may be nil (reflection disabled) — the Reflect endpoint degrades
// gracefully.
func NewLanesHandler(reg *memory.LaneRegistry, disp *memory.DispositionRecorder, recall *memory.RecallStore, defaultOperator string, reflector MemoryReflector) *LanesHandler {
	if defaultOperator == "" {
		defaultOperator = "operator"
	}
	return &LanesHandler{registry: reg, dispositions: disp, recall: recall, reflector: reflector, defaultOperator: defaultOperator}
}

// Routes mounts the lane endpoints under /api/memory/lanes. Mirrors the
// people-memory disposition surface (pending queue + approve/reject/retire/
// forget/defer/undo + truth recall) plus recall observability endpoints.
func (h *LanesHandler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/memory/lanes/pending", h.ListPending)
	mux.HandleFunc("GET /api/memory/lanes/truth", h.ListTruth)
	mux.HandleFunc("GET /api/memory/lanes/recall/events", h.RecallEvents)
	mux.HandleFunc("GET /api/memory/lanes/recall/ledger", h.RecallLedger)
	mux.HandleFunc("POST /api/memory/lanes/recall/{id}/outcome", h.MarkOutcome)
	mux.HandleFunc("POST /api/memory/lanes/recall/pull", h.PullRecall)
	mux.HandleFunc("GET /api/memory/lanes/search", h.Search)
	mux.HandleFunc("POST /api/memory/lanes/reflect", h.Reflect)
	mux.HandleFunc("POST /api/memory/lanes/{id}/approve", h.Approve)
	mux.HandleFunc("POST /api/memory/lanes/{id}/reject", h.Reject)
	mux.HandleFunc("POST /api/memory/lanes/{id}/modify", h.Modify)
	mux.HandleFunc("POST /api/memory/lanes/{id}/retire", h.Retire)
	mux.HandleFunc("POST /api/memory/lanes/{id}/forget", h.Forget)
	mux.HandleFunc("POST /api/memory/lanes/{id}/defer", h.Defer)
	mux.HandleFunc("POST /api/memory/lanes/{id}/undo", h.Undo)
	mux.HandleFunc("POST /api/memory/lanes/{id}/withdraw", h.Withdraw)
	mux.HandleFunc("POST /api/memory/lanes/{id}/link", h.Link)
	mux.HandleFunc("POST /api/memory/lanes/{id}/mark", h.Mark)
	mux.HandleFunc("GET /api/memory/lanes/{id}/graph", h.Graph)
	mux.HandleFunc("POST /api/memory/lanes/{id}/sensitivity", h.SetSensitivity)
	mux.HandleFunc("POST /api/memory/lanes/search/semantic", h.SemanticSearch)
	mux.HandleFunc("POST /api/memory/lanes/reindex", h.Reindex)
	mux.HandleFunc("GET /api/memory/lanes/cue/events", h.CueEvents)
	mux.HandleFunc("GET /api/memory/lanes/lifecycle", h.Lifecycle)
	return mux
}

// operatorFilter returns the operator query param (may be "").
func (h *LanesHandler) operatorFilter(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("operator"))
}

// requestOperator resolves the *acting* operator identity for a write.
// Precedence: X-Operator header (auth-derived identity) >
// ?operator= query > the configured defaultOperator (leader). This replaces the
// previously hard-coded leader name so multi-operator systems record the real
// actor on dispositions, edges, and recall events (Task #36).
func (h *LanesHandler) requestOperator(r *http.Request) string {
	if op := strings.TrimSpace(r.Header.Get("X-Operator")); op != "" {
		return op
	}
	if op := h.operatorFilter(r); op != "" {
		return op
	}
	return h.defaultOperator
}

// explicitOperator returns the operator *explicitly* supplied on the request
// (X-Operator header or ?operator= query), or "" when neither is present.
// Unlike requestOperator it does NOT fall back to defaultOperator, so callers
// can tell "operator was named" apart from "use the existing/default actor" —
// this prevents a non-scoped write from clobbering a record's owner/scope.
func (h *LanesHandler) explicitOperator(r *http.Request) string {
	if op := strings.TrimSpace(r.Header.Get("X-Operator")); op != "" {
		return op
	}
	return strings.TrimSpace(r.URL.Query().Get("operator"))
}

// visible reports whether an entry is visible to the requested operator under
// the 4-level sensitivity model + collection grant ACL (Gap2). "" = system
// scope (sees all); a named operator sees entries allowed by EntryVisible.
func visible(e *memory.LaneEntry, operator string) bool {
	return memory.EntryVisible(e, operator)
}

// ListPending returns pending (and deferred) candidate entries across every
// lane, newest first by submission order. With ?operator= it scopes to that
// operator's entries plus shared ones.
func (h *LanesHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	op := h.operatorFilter(r)
	out := make([]*memory.LaneEntry, 0)
	for _, t := range h.registry.LaneTypes() {
		for _, e := range h.registry.Lane(t).Pending() {
			if visible(e, op) {
				out = append(out, e)
			}
		}
		for _, e := range h.registry.Lane(t).Deferred() {
			if visible(e, op) {
				out = append(out, e)
			}
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// ListTruth returns approved (canonical) entries. With ?lane=<type> it scopes to
// one lane; with ?operator= it scopes to one operator's truth plus shared.
// Only approved entries are returned — pending candidates and
// retired/forgotten/deferred ones are excluded (M5 submission boundary).
func (h *LanesHandler) ListTruth(w http.ResponseWriter, r *http.Request) {
	laneParam := strings.TrimSpace(r.URL.Query().Get("lane"))
	sensitivity := strings.TrimSpace(r.URL.Query().Get("sensitivity"))
	op := h.operatorFilter(r)
	out := make([]*memory.LaneEntry, 0)
	if laneParam != "" {
		if l := h.registry.Lane(memory.LaneType(laneParam)); l != nil {
			for _, e := range l.Truth() {
				if visible(e, op) && matchesSensitivity(e, sensitivity) {
					out = append(out, e)
				}
			}
		}
	} else {
		for _, t := range h.registry.LaneTypes() {
			for _, e := range h.registry.Lane(t).Truth() {
				if visible(e, op) && matchesSensitivity(e, sensitivity) {
					out = append(out, e)
				}
			}
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// ---- Recall observability handlers (RecallEvents / RecallLedger / MarkOutcome
//      / PullRecall / Search / matchesSensitivity) live in
//      lanes_recall_handler.go (P0-1, P2-7, P1-5) ----

// dispose is the shared approval/reject/modify/retire/forget flow. It resolves
// the entry's owning lane, records the human disposition (which applies the
// status transition on the lane), and returns the updated entry.
func (h *LanesHandler) dispose(w http.ResponseWriter, r *http.Request, action memory.DispositionAction, modified string) {
	id := r.PathValue("id")
	laneType, ok := h.registry.FindLaneOf(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found"})
		return
	}
	op := h.requestOperator(r)
	if _, err := h.dispositions.Record(h.registry, id, laneType, action, modified, op); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	emitLaneDispositionTelemetry(action, laneType)
	entry, _ := h.registry.Lane(laneType).Get(id)
	// Append-only lifecycle trace (P1 three-axis / Task #39): approve/modify =
	// creation; retire/forget/withdraw/undo = correction.
	if h.recall != nil {
		switch action {
		case memory.DispositionAccept, memory.DispositionModify:
			h.recall.RecordLifecycle(memory.AxisCreation, id, string(laneType), string(action), memory.MaturityMeasured)
		case memory.DispositionRetire, memory.DispositionForget, memory.DispositionWithdraw, memory.DispositionUndo:
			h.recall.RecordLifecycle(memory.AxisCorrection, id, string(laneType), string(action), memory.MaturityMeasured)
		}
	}
	// Auto-embed approved truth (whole-entry vector) and its passages so it
	// becomes semantically searchable (Gap3 + P1 passage vectors). Fail-open.
	if (action == memory.DispositionAccept || action == memory.DispositionModify) && h.embedder != nil && entry != nil {
		if vec, eerr := h.embedder.Embed(r.Context(), entry.Content); eerr == nil {
			_ = h.registry.StoreVector(id, vec)
			_ = h.registry.StorePassages(id, entry.Content, func(p string) ([]float32, error) {
				return h.embedder.Embed(r.Context(), p)
			})
		}
	}
	respondJSON(w, http.StatusOK, entry)
}

// emitLaneDispositionTelemetry records the Shared Memory governance eval
// counter for a human disposition. fail-open: if telemetry is uninitialized the
// counter is simply skipped.
func emitLaneDispositionTelemetry(action memory.DispositionAction, lane memory.LaneType) {
	if !telemetry.IsInitialized() {
		return
	}
	var c metric.Int64Counter
	switch action {
	case memory.DispositionAccept, memory.DispositionModify:
		c = telemetry.LaneApproved
	case memory.DispositionReject:
		c = telemetry.LaneRejected
	case memory.DispositionRetire:
		c = telemetry.LaneRetired
	case memory.DispositionForget:
		c = telemetry.LaneForgotten
	case memory.DispositionDefer:
		c = telemetry.LaneDeferred
	case memory.DispositionUndo:
		c = telemetry.LaneUndone
	case memory.DispositionWithdraw:
		c = telemetry.LaneWithdrawn
	default:
		return
	}
	if c == nil {
		return
	}
	c.Add(context.Background(), 1, metric.WithAttributes(attribute.String("lane", string(lane))))
}

// Approve promotes a pending candidate to canonical truth.
func (h *LanesHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionAccept, "")
}

// Reject drops a pending candidate (it becomes forgotten, excluded from truth).
func (h *LanesHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionReject, "")
}

// Modify updates a candidate's content and approves it (canonical truth).
func (h *LanesHandler) Modify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}
	h.dispose(w, r, memory.DispositionModify, body.Content)
}

// Retire marks an approved entry as superseded (kept for audit, no longer truth).
func (h *LanesHandler) Retire(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionRetire, "")
}

// Forget hard-forgets an entry (excluded from recall).
func (h *LanesHandler) Forget(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionForget, "")
}

// Defer snoozes a pending candidate (decide later; not truth yet).
func (h *LanesHandler) Defer(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionDefer, "")
}

// Undo reverts the entry's most recent disposition (process-local).
func (h *LanesHandler) Undo(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionUndo, "")
}

// Withdraw re-opens an approved/deferred entry for re-disposition (P2-8).
func (h *LanesHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	h.dispose(w, r, memory.DispositionWithdraw, "")
}
