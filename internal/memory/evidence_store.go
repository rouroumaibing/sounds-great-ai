package memory

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryEvidenceStore implements EvidenceStore with in-memory data.
type InMemoryEvidenceStore struct {
	mu        sync.RWMutex
	evidence  []*EvidenceRecord
}

// NewEvidenceStore creates a new EvidenceStore implementation.
func NewEvidenceStore() EvidenceStore {
	return &InMemoryEvidenceStore{
		evidence: make([]*EvidenceRecord, 0),
	}
}

func (s *InMemoryEvidenceStore) ListEvidence() ([]*EvidenceRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*EvidenceRecord, len(s.evidence))
	copy(result, s.evidence)
	return result, nil
}

func (s *InMemoryEvidenceStore) AddEvidence(threadID, typ, title, content string, tags []string) (*EvidenceRecord, error) {
	if typ == "" {
		typ = "evidence"
	}
	rec := &EvidenceRecord{
		ID:        uuid.NewString(),
		ThreadID:  threadID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now().UnixMilli(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evidence = append(s.evidence, rec)
	return rec, nil
}
