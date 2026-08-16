package memory

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryEvidenceStore implements EvidenceStore. When created via
// NewEvidenceStoreAt it is backed by a JSON file and survives restarts
// (Persistent Identity layer: learned evidence is not lost on restart).
type InMemoryEvidenceStore struct {
	mu       sync.RWMutex
	path     string
	evidence []*EvidenceRecord
}

// NewEvidenceStore creates an in-memory (non-persistent) EvidenceStore.
func NewEvidenceStore() EvidenceStore {
	return &InMemoryEvidenceStore{
		evidence: make([]*EvidenceRecord, 0),
	}
}

// NewEvidenceStoreAt creates an EvidenceStore backed by a JSON file at path.
// Existing records are loaded on init; every AddEvidence persists atomically.
func NewEvidenceStoreAt(path string) EvidenceStore {
	s := &InMemoryEvidenceStore{path: path, evidence: make([]*EvidenceRecord, 0)}
	s.load()
	return s
}

func (s *InMemoryEvidenceStore) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("evidence: load failed (%s): %v", s.path, err)
		return
	}
	var recs []*EvidenceRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		log.Printf("evidence: corrupt store (%s): %v", s.path, err)
		return
	}
	s.evidence = recs
}

func (s *InMemoryEvidenceStore) flush() {
	if s.path == "" {
		return
	}
	data, err := json.MarshalIndent(s.evidence, "", "  ")
	if err != nil {
		return
	}
	if err := writeAtomic(s.path, data); err != nil {
		log.Printf("evidence: persist failed (%s): %v", s.path, err)
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
	s.flush()
	return rec, nil
}
