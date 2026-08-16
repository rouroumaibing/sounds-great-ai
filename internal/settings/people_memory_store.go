package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PeopleMemoryStore is the operator-scoped F276 contract. Both the file-backed
// (FilePeopleMemoryStore) and Redis-backed (RedisPeopleMemoryStore) stores
// satisfy it. Every method takes operatorID first: the entire
// people-memory keyspace is scoped by operatorID, and SG enforces that so multiple
// operators are isolated (KD-1 multi-operator).
type PeopleMemoryStore interface {
	// Proposal lifecycle (object 5).
	Propose(operatorID string, c *CaptureCandidate) (*CaptureCandidate, error)
	GetCandidate(operatorID, candidateID string) (*CaptureCandidate, bool, error)
	ListPending(operatorID string, limit int) ([]*CaptureCandidate, error)
	ApproveDrafts(operatorID, candidateID string, draftIDs []string) (*PersonMemoryDecisionReceipt, error)
	RejectDrafts(operatorID, candidateID string, draftIDs []string) (*CaptureCandidate, error)
	RejectCandidate(operatorID, candidateID string) (*CaptureCandidate, error)
	MarkNotNow(operatorID, candidateID string) (*CaptureCandidate, error)
	WithdrawCandidate(operatorID, candidateID string) (*CaptureCandidate, error)
	UndoDecision(operatorID, decisionID string) (*CaptureCandidate, error)

	// Lifecycle mutations on materialized truth.
	CorrectClaim(operatorID, personID, expectedCurrentClaimID string, payload PersonClaimPayload, src SourceRef) (*PersonClaimVersion, error)
	RetireClaim(operatorID, personID, expectedCurrentClaimID string, src SourceRef) error
	AmendInteraction(operatorID, personID, expectedEventID string, payload CandidateInteractionDraft, src SourceRef) (*InteractionEvent, error)
	RedactItem(operatorID, personID string, item RedactTarget) error
	HardForget(operatorID, personID string) (*PersonMemoryDeletionReceipt, error)
	HardForgetProposal(operatorID, proposalID string) (*PersonMemoryDeletionReceipt, error)

	// Read / derive surface.
	RecallCard(operatorID, personID string) (*RelationshipCard, bool, error)
	// RecallContextForQuery returns a token-bounded "关系记忆" block for the
	// given user message (homologous anchor-first recall injection), or
	// ("", false) when no known person is referenced.
	RecallContextForQuery(operatorID, query string) (string, error)
	// RecallDrill returns the verbatim backing of one recall item (claim/
	// relationship/event), enforcing homologous per-turn drill budgets.
	// It is read-only: it never mutates the persisted document, only the
	// ephemeral (operator, turn) budget map held by the store.
	RecallDrill(operatorID string, input PeopleMemoryDrillInput) (*PeopleMemoryDrillResult, error)
	ResolveActivePersonByAlias(operatorID, alias string) (string, error)
	ListPeople(operatorID string) ([]*PersonIdentity, error)
	GetPerson(operatorID, personID string) (*PersonIdentity, bool, error)
	ListClaims(operatorID, personID string) ([]*PersonClaimVersion, error)
	ListRelationships(operatorID, personID string) ([]*PersonRelationship, error)
	ListEvents(operatorID, personID string) ([]*InteractionEvent, error)

	// Dual-path deferred receipts.
	DeferReceipt(operatorID, requesterCat, subject, personID string, coords []SourceRef) (*DeferredPersonMemoryReceipt, error)
	ListReadyDeferred(operatorID string) ([]*DeferredPersonMemoryReceipt, error)
	ClaimDeferredReceipt(operatorID, receiptID, requesterCat string) (*CaptureCandidate, error)
	// ReserveDeferredReceipt marks a receipt claimed without creating a candidate
	// (used by the daily clerk before re-invoking the original dog). ReleaseDeferredReceipt
	// clears the reservation so the receipt becomes ready again.
	ReserveDeferredReceipt(operatorID, receiptID, by string) error
	ReleaseDeferredReceipt(operatorID, receiptID string) error
	WithdrawReceipt(operatorID, receiptID string) error
	ForgetReceipt(operatorID, receiptID string) error

	// ListOperators returns the operator ids that currently hold any data
	// (used by the daily clerk to iterate scopes).
	ListOperators() ([]string, error)
}

// FilePeopleMemoryStore is the zero-dependency, owner-private F276 store. It is
// safe for concurrent use (one RWMutex guards the whole operator map; all
// mutations persist atomically through writeAtomicRaw). The on-disk envelope is
// a map[operatorID]*peopleMemoryDocument under ConfigRoot/people-memory.json.
type FilePeopleMemoryStore struct {
	path     string
	mu       sync.RWMutex
	operators map[string]*peopleMemoryDocument
	// drillBudgets is the ephemeral (operator, turn) -> spend map for on-demand
	// drill budgeting. Never persisted (see drillTurnBudget).
	drillBudgets map[string]*drillTurnBudget
}

// NewFilePeopleMemoryStore opens (or creates) the owner-private people-memory
// document under configRoot. The directory is created if absent.
func NewFilePeopleMemoryStore(configRoot string) *FilePeopleMemoryStore {
	path := filepath.Join(configRoot, PeopleMemoryFileName)
	_ = os.MkdirAll(configRoot, 0o755)
	s := &FilePeopleMemoryStore{path: path, operators: map[string]*peopleMemoryDocument{}, drillBudgets: map[string]*drillTurnBudget{}}
	s.load()
	return s
}

func (s *FilePeopleMemoryStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.operators = map[string]*peopleMemoryDocument{}
			return
		}
		s.operators = map[string]*peopleMemoryDocument{}
		return
	}
	docs := map[string]*peopleMemoryDocument{}
	if err := json.Unmarshal(raw, &docs); err != nil || docs == nil {
		s.operators = map[string]*peopleMemoryDocument{}
		return
	}
	s.operators = docs
}

func (s *FilePeopleMemoryStore) save() error {
	data, err := json.MarshalIndent(s.operators, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal people-memory: %w", err)
	}
	return writeAtomicRaw(s.path, data, 0o644)
}

// opDoc returns the document for an operator, creating an empty one if absent.
// Caller MUST hold s.mu (write).
func (s *FilePeopleMemoryStore) opDoc(operatorID string) *peopleMemoryDocument {
	if operatorID == "" {
		operatorID = "operator"
	}
	d, ok := s.operators[operatorID]
	if !ok {
		d = newPeopleMemoryDocument()
		s.operators[operatorID] = d
	}
	return d
}

// ListOperators returns operator ids that hold data.
func (s *FilePeopleMemoryStore) ListOperators() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.operators))
	for op := range s.operators {
		if len(s.operators[op].People) > 0 || len(s.operators[op].Candidates) > 0 || len(s.operators[op].Receipts) > 0 {
			out = append(out, op)
		}
	}
	return out, nil
}

// ---- Proposal lifecycle wrappers (thin: lock, scope doc, persist) ----

func (s *FilePeopleMemoryStore) Propose(operatorID string, c *CaptureCandidate) (*CaptureCandidate, error) {
	if c == nil {
		return nil, fmt.Errorf("nil candidate")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	stored, err := d.propose(c)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *FilePeopleMemoryStore) GetCandidate(operatorID, candidateID string) (*CaptureCandidate, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	c, ok := d.Candidates[candidateID]
	return c, ok, nil
}

func (s *FilePeopleMemoryStore) ListPending(operatorID string, limit int) ([]*CaptureCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listPending(limit), nil
}

func (s *FilePeopleMemoryStore) ApproveDrafts(operatorID, candidateID string, draftIDs []string) (*PersonMemoryDecisionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.approveDrafts(candidateID, draftIDs)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *FilePeopleMemoryStore) RejectDrafts(operatorID, candidateID string, draftIDs []string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.rejectDrafts(candidateID, draftIDs)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *FilePeopleMemoryStore) RejectCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.rejectCandidate(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *FilePeopleMemoryStore) MarkNotNow(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.markNotNow(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *FilePeopleMemoryStore) WithdrawCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.withdrawCandidate(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *FilePeopleMemoryStore) UndoDecision(operatorID, decisionID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.undoDecision(decisionID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

// ---- Lifecycle wrappers ----

func (s *FilePeopleMemoryStore) CorrectClaim(operatorID, personID, expectedCurrentClaimID string, payload PersonClaimPayload, src SourceRef) (*PersonClaimVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	cv, err := d.correctClaim(personID, expectedCurrentClaimID, payload, src)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return cv, nil
}

func (s *FilePeopleMemoryStore) RetireClaim(operatorID, personID, expectedCurrentClaimID string, src SourceRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.retireClaim(personID, expectedCurrentClaimID, src); err != nil {
		return err
	}
	return s.save()
}

func (s *FilePeopleMemoryStore) AmendInteraction(operatorID, personID, expectedEventID string, payload CandidateInteractionDraft, src SourceRef) (*InteractionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	ev, err := d.amendInteraction(personID, expectedEventID, payload, src)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return ev, nil
}

func (s *FilePeopleMemoryStore) RedactItem(operatorID, personID string, item RedactTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.redactItem(personID, item); err != nil {
		return err
	}
	return s.save()
}

func (s *FilePeopleMemoryStore) HardForget(operatorID, personID string) (*PersonMemoryDeletionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.hardForget(personID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *FilePeopleMemoryStore) HardForgetProposal(operatorID, proposalID string) (*PersonMemoryDeletionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.hardForgetProposal(proposalID)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return rec, nil
}

// ---- Read / derive wrappers ----

func (s *FilePeopleMemoryStore) RecallCard(operatorID, personID string) (*RelationshipCard, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.recallCard(personID)
}

func (s *FilePeopleMemoryStore) ResolveActivePersonByAlias(operatorID, alias string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.resolveActivePersonByAlias(alias)
}

func (s *FilePeopleMemoryStore) RecallContextForQuery(operatorID, query string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	block, _ := d.recallContextForQuery(query)
	return block, nil
}

// RecallDrill implements the on-demand drill with homologous per-turn
// budget discipline. It is read-only against the persisted document; it only
// mutates the ephemeral drillBudgets map.
func (s *FilePeopleMemoryStore) RecallDrill(operatorID string, input PeopleMemoryDrillInput) (*PeopleMemoryDrillResult, error) {
	if operatorID == "" {
		operatorID = "operator"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)

	// Person must be active (ownerUserId match + status === active).
	p, ok := d.People[input.PersonID]
	if !ok || p.Status != PersonStatusActive {
		return &PeopleMemoryDrillResult{Status: "not_available"}, nil
	}
	// Time window validation (finite + from <= to).
	if input.TimeWindowFrom <= 0 || input.TimeWindowTo <= 0 || input.TimeWindowFrom > input.TimeWindowTo {
		return &PeopleMemoryDrillResult{Status: "not_available"}, nil
	}
	text, src, recordedAt, found := d.drillFindItem(input)
	if !found {
		return &PeopleMemoryDrillResult{Status: "not_available"}, nil
	}
	if recordedAt < input.TimeWindowFrom || recordedAt > input.TimeWindowTo {
		return &PeopleMemoryDrillResult{Status: "not_available"}, nil
	}

	// Per-turn, per-person budget enforcement (PersonMemoryRecallService.drill).
	turnKey := operatorID + "\x00" + input.TurnID
	budget := s.drillBudgets[turnKey]
	if budget == nil {
		if len(s.drillBudgets) > peopleMemoryDrillBudgetCap {
			s.drillBudgets = map[string]*drillTurnBudget{}
		}
		budget = &drillTurnBudget{aggregateTokens: 0, callsByPerson: map[string]int{}}
		s.drillBudgets[turnKey] = budget
	}
	calls := budget.callsByPerson[input.PersonID]
	if calls >= PeopleMemoryDrillMaxPerPersonPerTurn {
		return &PeopleMemoryDrillResult{Status: "budget_exceeded"}, nil
	}
	bounded, tk := boundedProjectionText(text)
	if budget.aggregateTokens+tk > PeopleMemoryDrillMaxAggregatePerTurn {
		return &PeopleMemoryDrillResult{Status: "budget_exceeded"}, nil
	}
	budget.callsByPerson[input.PersonID] = calls + 1
	budget.aggregateTokens += tk
	return &PeopleMemoryDrillResult{
		Status:         "ok",
		Kind:           input.ItemKind,
		ItemID:         input.ItemID,
		Text:           bounded,
		SourceRef:      src,
		EstimatedTokens: tk,
	}, nil
}

func (s *FilePeopleMemoryStore) ListPeople(operatorID string) ([]*PersonIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listPeople(), nil
}

func (s *FilePeopleMemoryStore) GetPerson(operatorID, personID string) (*PersonIdentity, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	p, ok := d.People[personID]
	return p, ok, nil
}

func (s *FilePeopleMemoryStore) ListClaims(operatorID, personID string) ([]*PersonClaimVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listClaims(personID), nil
}

func (s *FilePeopleMemoryStore) ListRelationships(operatorID, personID string) ([]*PersonRelationship, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listRelationships(personID), nil
}

func (s *FilePeopleMemoryStore) ListEvents(operatorID, personID string) ([]*InteractionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listEvents(personID), nil
}

// ---- Dual-path deferred receipts (file: stored inside the operator doc) ----

func (s *FilePeopleMemoryStore) DeferReceipt(operatorID, requesterCat, subject, personID string, coords []SourceRef) (*DeferredPersonMemoryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	r, err := d.deferReceipt(operatorID, requesterCat, subject, personID, coords)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *FilePeopleMemoryStore) ListReadyDeferred(operatorID string) ([]*DeferredPersonMemoryReceipt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.opDoc(operatorID)
	return d.listReadyDeferred(), nil
}

func (s *FilePeopleMemoryStore) ClaimDeferredReceipt(operatorID, receiptID, requesterCat string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.claimDeferredReceipt(receiptID, requesterCat)
	if err != nil {
		return nil, err
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *FilePeopleMemoryStore) WithdrawReceipt(operatorID, receiptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.withdrawReceipt(receiptID); err != nil {
		return err
	}
	return s.save()
}

func (s *FilePeopleMemoryStore) ReserveDeferredReceipt(operatorID, receiptID, by string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.reserveDeferredReceipt(receiptID, by); err != nil {
		return err
	}
	return s.save()
}

func (s *FilePeopleMemoryStore) ReleaseDeferredReceipt(operatorID, receiptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.releaseDeferredReceipt(receiptID); err != nil {
		return err
	}
	return s.save()
}

func (s *FilePeopleMemoryStore) ForgetReceipt(operatorID, receiptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.forgetReceipt(receiptID); err != nil {
		return err
	}
	return s.save()
}
