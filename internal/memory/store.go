package memory

import (
	"strings"
	"sync"
)

type Evidence struct {
	ID      string
	Breed   string
	Content string
	Task    string
}

type Lesson struct {
	ID      string
	Content string
	Context string
}

type Decision struct {
	ID       string
	Topic    string
	Decision string
	Reason   string
}

type MemoryStore struct {
	mu        sync.RWMutex
	evidence  []Evidence
	lessons   []Lesson
	decisions map[string]*Decision
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{decisions: make(map[string]*Decision)}
}

func (s *MemoryStore) AddEvidence(e Evidence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evidence = append(s.evidence, e)
}

func (s *MemoryStore) QueryEvidence(query string) []Evidence {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []Evidence
	for _, e := range s.evidence {
		if strings.Contains(strings.ToLower(e.Content), strings.ToLower(query)) {
			results = append(results, e)
		}
	}
	return results
}

func (s *MemoryStore) AddDecision(d Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[d.ID] = &d
}

func (s *MemoryStore) GetDecision(id string) *Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.decisions[id]
}
