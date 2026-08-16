package settings

import (
	"context"
	"fmt"
	"strings"
)

// This file holds the thin, operator-scoped wrappers of RedisPeopleMemoryStore
// for the lower-contention paths (propose/approve/recall/lifecycle). They reuse
// the shared document methods and persist the operator doc atomically to Redis.

func (s *RedisPeopleMemoryStore) GetCandidate(operatorID, candidateID string) (*CaptureCandidate, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, ok := d.Candidates[candidateID]
	return c, ok, nil
}

func (s *RedisPeopleMemoryStore) ListPending(operatorID string, limit int) ([]*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.listPending(limit), nil
}

func (s *RedisPeopleMemoryStore) Propose(operatorID string, c *CaptureCandidate) (*CaptureCandidate, error) {
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
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *RedisPeopleMemoryStore) ApproveDrafts(operatorID, candidateID string, draftIDs []string) (*PersonMemoryDecisionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.approveDrafts(candidateID, draftIDs)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *RedisPeopleMemoryStore) RejectDrafts(operatorID, candidateID string, draftIDs []string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.rejectDrafts(candidateID, draftIDs)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) RejectCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.rejectCandidate(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) MarkNotNow(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.markNotNow(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) WithdrawCandidate(operatorID, candidateID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.withdrawCandidate(candidateID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) UndoDecision(operatorID, decisionID string) (*CaptureCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	c, err := d.undoDecision(decisionID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) CorrectClaim(operatorID, personID, expectedCurrentClaimID string, payload PersonClaimPayload, src SourceRef) (*PersonClaimVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	cv, err := d.correctClaim(personID, expectedCurrentClaimID, payload, src)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return cv, nil
}

func (s *RedisPeopleMemoryStore) RetireClaim(operatorID, personID, expectedCurrentClaimID string, src SourceRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.retireClaim(personID, expectedCurrentClaimID, src); err != nil {
		return err
	}
	return s.persist(operatorID)
}

func (s *RedisPeopleMemoryStore) AmendInteraction(operatorID, personID, expectedEventID string, payload CandidateInteractionDraft, src SourceRef) (*InteractionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	ev, err := d.amendInteraction(personID, expectedEventID, payload, src)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return ev, nil
}

func (s *RedisPeopleMemoryStore) RedactItem(operatorID, personID string, item RedactTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	if err := d.redactItem(personID, item); err != nil {
		return err
	}
	return s.persist(operatorID)
}

// HardForget overrides the document method to also purge any Redis-stored
// deferred receipts bound to the person (the doc map holds none in Redis mode).
func (s *RedisPeopleMemoryStore) HardForget(operatorID, personID string) (*PersonMemoryDeletionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.hardForget(personID)
	if err != nil {
		return nil, err
	}
	if err := s.purgeRedisReceiptsForPerson(operatorID, personID); err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *RedisPeopleMemoryStore) purgeRedisReceiptsForPerson(operatorID, personID string) error {
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(context.Background(), cursor, s.rcptKey(operatorID, "*"), 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			id := strings.TrimPrefix(key, s.rcptKey(operatorID, ""))
			r, ok := s.readReceipt(operatorID, id)
			if !ok || r.PersonID != personID {
				continue
			}
			if derr := s.client.Del(context.Background(), key).Err(); derr != nil {
				return derr
			}
			if derr := s.client.Del(context.Background(), s.coordsKey(operatorID, id)).Err(); derr != nil {
				return derr
			}
			if zerr := s.client.ZRem(context.Background(), s.readyKey(operatorID), id).Err(); zerr != nil {
				return zerr
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (s *RedisPeopleMemoryStore) HardForgetProposal(operatorID, proposalID string) (*PersonMemoryDeletionReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	rec, err := d.hardForgetProposal(proposalID)
	if err != nil {
		return nil, err
	}
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *RedisPeopleMemoryStore) RecallCard(operatorID, personID string) (*RelationshipCard, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.recallCard(personID)
}

func (s *RedisPeopleMemoryStore) ResolveActivePersonByAlias(operatorID, alias string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.resolveActivePersonByAlias(alias)
}

func (s *RedisPeopleMemoryStore) RecallContextForQuery(operatorID, query string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	block, _ := d.recallContextForQuery(query)
	return block, nil
}

// RecallDrill implements the on-demand drill with homologous per-turn
// budget discipline. It is read-only against the persisted document; it only
// mutates the ephemeral drillBudgets map (held per-process, like the
// in-memory PersonMemoryRecallService.budgets).
func (s *RedisPeopleMemoryStore) RecallDrill(operatorID string, input PeopleMemoryDrillInput) (*PeopleMemoryDrillResult, error) {
	if operatorID == "" {
		operatorID = "operator"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)

	p, ok := d.People[input.PersonID]
	if !ok || p.Status != PersonStatusActive {
		return &PeopleMemoryDrillResult{Status: "not_available"}, nil
	}
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
		Status:          "ok",
		Kind:            input.ItemKind,
		ItemID:          input.ItemID,
		Text:            bounded,
		SourceRef:       src,
		EstimatedTokens: tk,
	}, nil
}

func (s *RedisPeopleMemoryStore) ListPeople(operatorID string) ([]*PersonIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.listPeople(), nil
}

func (s *RedisPeopleMemoryStore) GetPerson(operatorID, personID string) (*PersonIdentity, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	p, ok := d.People[personID]
	return p, ok, nil
}

func (s *RedisPeopleMemoryStore) ListClaims(operatorID, personID string) ([]*PersonClaimVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.listClaims(personID), nil
}

func (s *RedisPeopleMemoryStore) ListRelationships(operatorID, personID string) ([]*PersonRelationship, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.listRelationships(personID), nil
}

func (s *RedisPeopleMemoryStore) ListEvents(operatorID, personID string) ([]*InteractionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	return d.listEvents(personID), nil
}
