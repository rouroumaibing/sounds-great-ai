package dossier

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// In-memory fallbacks used when SQLitePath is empty (tests, disposable
// runs) — mirroring the threadstore factory convention.

type memObservationStore struct {
	mu    sync.Mutex
	items map[string]Observation
	order []string
}

// NewMemoryObservationStore creates an in-memory ObservationStore.
func NewMemoryObservationStore() ObservationStore {
	return &memObservationStore{items: make(map[string]Observation)}
}

func (s *memObservationStore) Add(input AddObservationInput) (Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	obs := Observation{
		ID:         fmt.Sprintf("obs_%d_%d", now.UnixMilli(), len(s.order)),
		DogID:      input.DogID,
		Content:    input.Content,
		Provenance: ObservationProvenance{Type: "operator", Author: input.Author, Date: now.Format("2006-01-02")},
		CreatedAt:  now,
	}
	s.items[obs.ID] = obs
	s.order = append(s.order, obs.ID)
	return obs, nil
}

func (s *memObservationStore) List(dogID string, limit int) ([]Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = clampLimit(limit)
	var out []Observation
	for i := len(s.order) - 1; i >= 0 && len(out) < limit; i-- {
		if obs := s.items[s.order[i]]; obs.DogID == dogID {
			out = append(out, obs)
		}
	}
	return out, nil
}

func (s *memObservationStore) ListAll(limit int) (map[string][]Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = clampLimit(limit)
	grouped := make(map[string][]Observation)
	counts := make(map[string]int)
	for i := len(s.order) - 1; i >= 0; i-- {
		obs := s.items[s.order[i]]
		if counts[obs.DogID] >= limit {
			continue
		}
		counts[obs.DogID]++
		grouped[obs.DogID] = append(grouped[obs.DogID], obs)
	}
	return grouped, nil
}

func (s *memObservationStore) Get(id string) (Observation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	obs, ok := s.items[id]
	return obs, ok, nil
}

type memProposalStore struct {
	mu       sync.Mutex
	items    map[string]DistillationProposal
	bySource map[string]string
	order    []string
	counter  int
}

// NewMemoryProposalStore creates an in-memory ProposalStore.
func NewMemoryProposalStore() ProposalStore {
	return &memProposalStore{items: make(map[string]DistillationProposal), bySource: make(map[string]string)}
}

func (s *memProposalStore) Create(in CreateProposalInput) (DistillationProposal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.bySource[in.SourceID]; ok {
		return s.items[id], false, nil
	}
	s.counter++
	now := time.Now()
	p := DistillationProposal{
		ProposalID:     fmt.Sprintf("dsp_%d_%d", now.UnixMilli(), s.counter),
		Status:         ProposalPending,
		SourceEvent:    in.SourceEvent,
		SourceID:       in.SourceID,
		TargetDogID:    in.TargetDogID,
		TargetFields:   in.TargetFields,
		BeforeSnapshot: in.BeforeSnapshot,
		AfterDraft:     in.AfterDraft,
		Rationale:      in.Rationale,
		EvidenceRefs:   in.EvidenceRefs,
		BaseHash:       in.BaseHash,
		CreatedAt:      now,
		CreatedBy:      in.CreatedBy,
	}
	s.items[p.ProposalID] = p
	s.bySource[p.SourceID] = p.ProposalID
	s.order = append(s.order, p.ProposalID)
	return p, true, nil
}

func (s *memProposalStore) Get(id string) (DistillationProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.items[id]; ok {
		return p, nil
	}
	return DistillationProposal{}, ErrProposalNotFound
}

func (s *memProposalStore) GetBySourceID(sourceID string) (DistillationProposal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.bySource[sourceID]; ok {
		return s.items[id], true, nil
	}
	return DistillationProposal{}, false, nil
}

func (s *memProposalStore) ListPending(limit int) ([]DistillationProposal, error) {
	return s.listFilter(func(p DistillationProposal) bool { return p.Status == ProposalPending }, limit)
}

func (s *memProposalStore) ListByDog(dogID string, limit int) ([]DistillationProposal, error) {
	return s.listFilter(func(p DistillationProposal) bool { return p.TargetDogID == dogID }, limit)
}

func (s *memProposalStore) listFilter(match func(DistillationProposal) bool, limit int) ([]DistillationProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var out []DistillationProposal
	for i := len(s.order) - 1; i >= 0 && len(out) < limit; i-- {
		if p := s.items[s.order[i]]; match(p) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *memProposalStore) MarkApproved(id, approvedBy string) (DistillationProposal, error) {
	return s.transition(id, ProposalPending, func(p *DistillationProposal) {
		p.Status = ProposalApproved
		p.ApprovedBy = approvedBy
		now := time.Now()
		p.ApprovedAt = &now
	})
}

func (s *memProposalStore) MarkRejected(id, rejectedBy, reason string) (DistillationProposal, error) {
	return s.transition(id, ProposalPending, func(p *DistillationProposal) {
		p.Status = ProposalRejected
		p.RejectedBy = rejectedBy
		p.RejectReason = reason
		now := time.Now()
		p.RejectedAt = &now
	})
}

func (s *memProposalStore) MarkApplied(id, appliedBy, commitSHA string) (DistillationProposal, error) {
	return s.transition(id, ProposalApproved, func(p *DistillationProposal) {
		p.Status = ProposalApplied
		p.AppliedBy = appliedBy
		p.AppliedCommitSHA = commitSHA
		now := time.Now()
		p.AppliedAt = &now
	})
}

func (s *memProposalStore) transition(id string, from ProposalStatus, mutate func(*DistillationProposal)) (DistillationProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.items[id]
	if !ok {
		return DistillationProposal{}, ErrProposalNotFound
	}
	if p.Status != from {
		return DistillationProposal{}, fmt.Errorf("%w: proposal %s is %s, expected %s", ErrProposalState, id, p.Status, from)
	}
	mutate(&p)
	s.items[id] = p
	return p, nil
}

// Store factories (SQLite when path set, in-memory otherwise).

// NewObservationStoreAt is the factory matching the threadstore convention.
func NewObservationStoreAt(sqlitePath string) (ObservationStore, error) {
	if sqlitePath == "" {
		return NewMemoryObservationStore(), nil
	}
	return NewSQLiteObservationStore(sqlitePath)
}

// NewProposalStoreAt is the factory matching the threadstore convention.
func NewProposalStoreAt(sqlitePath string) (ProposalStore, error) {
	if sqlitePath == "" {
		return NewMemoryProposalStore(), nil
	}
	return NewSQLiteProposalStore(sqlitePath)
}
