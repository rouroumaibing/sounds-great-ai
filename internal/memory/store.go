package memory

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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

// memoryDocument is the on-disk envelope for a persistent MemoryStore.
type memoryDocument struct {
	Evidence  []Evidence           `json:"evidence"`
	Lessons   []Lesson             `json:"lessons"`
	Decisions map[string]*Decision `json:"decisions"`
}

type MemoryStore struct {
	mu        sync.RWMutex
	path      string
	evidence  []Evidence
	lessons   []Lesson
	decisions map[string]*Decision
}

// NewMemoryStore returns an in-memory (non-persistent) store. Use for tests
// and callers that do not need durability across restarts.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{decisions: make(map[string]*Decision)}
}

// NewMemoryStoreAt returns a store backed by a JSON file at path. Existing
// data is loaded on init; every mutation is persisted atomically (tmp+rename).
// This is the Persistent Identity layer (F231/F287):
// learned experience (evidence/lessons/decisions) survives restarts instead of
// being lost with the process.
func NewMemoryStoreAt(path string) *MemoryStore {
	s := &MemoryStore{path: path, decisions: make(map[string]*Decision)}
	s.load()
	return s
}

func (s *MemoryStore) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("memory: load failed (%s): %v", s.path, err)
		return
	}
	var doc memoryDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		log.Printf("memory: corrupt store (%s): %v", s.path, err)
		return
	}
	s.evidence = doc.Evidence
	s.lessons = doc.Lessons
	if doc.Decisions != nil {
		s.decisions = doc.Decisions
	}
}

func (s *MemoryStore) flush() {
	if s.path == "" {
		return
	}
	doc := memoryDocument{
		Evidence:  s.evidence,
		Lessons:   s.lessons,
		Decisions: s.decisions,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return
	}
	if err := writeAtomic(s.path, data); err != nil {
		log.Printf("memory: persist failed (%s): %v", s.path, err)
	}
}

func (s *MemoryStore) AddEvidence(e Evidence) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evidence = append(s.evidence, e)
	s.flush()
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

// AddLesson persists a learned lesson. Lessons are part of the experience
// memory that must survive restarts (Persistent Identity layer).
func (s *MemoryStore) AddLesson(l Lesson) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lessons = append(s.lessons, l)
	s.flush()
}

func (s *MemoryStore) AddDecision(d Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions[d.ID] = &d
	s.flush()
}

func (s *MemoryStore) GetDecision(id string) *Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.decisions[id]
}

// writeAtomic writes data to path via a temp file + atomic rename, creating
// the parent directory if needed. Mirrors the durability guarantee used by
// internal/settings (tmp+rename, never a half-written file).
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
