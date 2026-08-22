package dossier

import (
	"fmt"
	"sync"
	"time"
)

// DistillationSourceEvent enumerates the events allowed to trigger a
// distillation opportunity. Deliberately narrow: only stable, verifiable
// lifecycle events (plus "manual" for operator-initiated cold starts).
const (
	SourceEventReviewComplete = "review-complete"
	SourceEventFeatPhaseClose = "feat-phase-close"
	SourceEventManual         = "manual"
)

// DistillationSourceEvents is the validation whitelist.
var DistillationSourceEvents = []string{SourceEventReviewComplete, SourceEventFeatPhaseClose, SourceEventManual}

// IsDistillationSourceEvent checks membership in the whitelist.
func IsDistillationSourceEvent(v string) bool {
	for _, e := range DistillationSourceEvents {
		if e == v {
			return true
		}
	}
	return false
}

// DistillationOpportunity is a transient workflow signal telling a dog
// "a capability-relevant event just closed; consider distilling it into
// your dossier". Opportunities are deliberately NOT persisted: they are
// prompts, not ledgers. Losing one to a restart is acceptable; the durable
// artifacts are observations (SQLite) and proposals (SQLite).
type DistillationOpportunity struct {
	OpportunityID string    `json:"opportunityId"`
	SourceEvent   string    `json:"sourceEvent"`
	SourceID      string    `json:"sourceId"` // idempotency key, e.g. "review-complete:{thread}:{sha}:{reviewer}"
	TargetDogID   string    `json:"targetDogId"` // who should consider distilling (the PR author)
	ThreadID      string    `json:"threadId"`
	ReviewerDogID string    `json:"reviewerDogId"`
	AuthorDogID   string    `json:"authorDogId"`
	Status        string    `json:"status"` // pending | converted | dismissed
	CreatedAt     time.Time `json:"createdAt"`
	// ConvertedToProposalID is set when status = converted.
	ConvertedToProposalID string `json:"convertedToProposalId,omitempty"`
}

// ReviewCompleteContext describes one finished review cycle.
type ReviewCompleteContext struct {
	ThreadID      string
	ReviewerDogID string // who reviewed
	AuthorDogID   string // who was reviewed — distillation target
	CommitSHA     string // reviewed artifact anchor ("" if unknown)
}

// OpportunityStore is the in-memory opportunity port.
type OpportunityStore interface {
	GetBySourceID(sourceID string) (DistillationOpportunity, bool)
	Create(op DistillationOpportunity) (DistillationOpportunity, error)
	ListPending() []DistillationOpportunity
	Dismiss(id string) bool
	MarkConverted(id, proposalID string) bool
}

// InMemoryOpportunityStore is the dev/test/default implementation.
type InMemoryOpportunityStore struct {
	mu       sync.Mutex
	items    map[string]DistillationOpportunity
	bySource map[string]string // sourceID → opportunityID
	counter  int
}

// NewInMemoryOpportunityStore creates an empty opportunity store.
func NewInMemoryOpportunityStore() *InMemoryOpportunityStore {
	return &InMemoryOpportunityStore{
		items:    make(map[string]DistillationOpportunity),
		bySource: make(map[string]string),
	}
}

// GetBySourceID returns the opportunity recorded for a sourceID, if any.
func (s *InMemoryOpportunityStore) GetBySourceID(sourceID string) (DistillationOpportunity, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.bySource[sourceID]
	if !ok {
		return DistillationOpportunity{}, false
	}
	op, ok := s.items[id]
	return op, ok
}

// Create stores a new opportunity.
func (s *InMemoryOpportunityStore) Create(op DistillationOpportunity) (DistillationOpportunity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	op.OpportunityID = fmt.Sprintf("opp-%d", s.counter)
	if op.CreatedAt.IsZero() {
		op.CreatedAt = time.Now()
	}
	op.Status = "pending"
	s.items[op.OpportunityID] = op
	s.bySource[op.SourceID] = op.OpportunityID
	return op, nil
}

// ListPending returns all pending opportunities.
func (s *InMemoryOpportunityStore) ListPending() []DistillationOpportunity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DistillationOpportunity, 0, len(s.items))
	for _, op := range s.items {
		if op.Status == "pending" {
			out = append(out, op)
		}
	}
	return out
}

// Dismiss marks an opportunity dismissed. Returns false if not found/processed.
func (s *InMemoryOpportunityStore) Dismiss(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.items[id]
	if !ok || op.Status != "pending" {
		return false
	}
	op.Status = "dismissed"
	s.items[id] = op
	return true
}

// MarkConverted marks an opportunity converted into a proposal.
func (s *InMemoryOpportunityStore) MarkConverted(id, proposalID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	op, ok := s.items[id]
	if !ok || op.Status != "pending" {
		return false
	}
	op.Status = "converted"
	op.ConvertedToProposalID = proposalID
	s.items[id] = op
	return true
}

// Checkpoint records distillation opportunities on lifecycle events. The
// service never decides proposal content — dogs retain judgment over whether
// and what to propose (summary layer must be dog/peer/operator judged, never
// algorithm-generated).
type Checkpoint struct {
	store OpportunityStore
	// log receives best-effort diagnostics; nil-safe.
	log func(format string, args ...any)

	mu       sync.Mutex
	inFlight map[string]struct{}
}

// NewCheckpoint creates the checkpoint service.
func NewCheckpoint(store OpportunityStore, log func(format string, args ...any)) *Checkpoint {
	if log == nil {
		log = func(string, ...any) {}
	}
	return &Checkpoint{store: store, log: log, inFlight: make(map[string]struct{})}
}

// OnReviewComplete records a review-complete opportunity targeting the
// reviewed author. Idempotent per (thread, sha, reviewer); in-flight
// duplicate calls coalesce.
func (c *Checkpoint) OnReviewComplete(ctx ReviewCompleteContext) (string, bool) {
	sha := ctx.CommitSHA
	if sha == "" {
		sha = "nosha"
	}
	sourceID := fmt.Sprintf("%s:%s:%s:%s", SourceEventReviewComplete, ctx.ThreadID, sha, ctx.ReviewerDogID)
	return c.record(sourceID, SourceEventReviewComplete, ctx.ThreadID, ctx.ReviewerDogID, ctx.AuthorDogID)
}

func (c *Checkpoint) record(sourceID, sourceEvent, threadID, reviewerDogID, targetDogID string) (string, bool) {
	if _, ok := c.store.GetBySourceID(sourceID); ok {
		c.log("[dossier-checkpoint] already recorded: %s", sourceID)
		return sourceID, false
	}

	c.mu.Lock()
	if _, busy := c.inFlight[sourceID]; busy {
		c.mu.Unlock()
		return sourceID, false
	}
	c.inFlight[sourceID] = struct{}{}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.inFlight, sourceID)
		c.mu.Unlock()
	}()

	// Re-check after acquiring the in-flight slot (another goroutine may
	// have completed while we waited — cheap map reads, no harm).
	if _, ok := c.store.GetBySourceID(sourceID); ok {
		return sourceID, false
	}
	if _, err := c.store.Create(DistillationOpportunity{
		SourceEvent:   sourceEvent,
		SourceID:      sourceID,
		TargetDogID:   targetDogID,
		ThreadID:      threadID,
		ReviewerDogID: reviewerDogID,
		AuthorDogID:   targetDogID,
	}); err != nil {
		c.log("[dossier-checkpoint] create failed for %s: %v", sourceID, err)
		return sourceID, false
	}
	c.log("[dossier-checkpoint] opportunity created: %s → %s", sourceID, targetDogID)
	return sourceID, true
}
