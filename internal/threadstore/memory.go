package threadstore

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryThreadStore implements ThreadStore with in-memory data structures.
type InMemoryThreadStore struct {
	mu       sync.RWMutex
	threads  map[string]*Thread
	events   map[string][]json.RawMessage
	sessions map[string]*SessionRecord
}

// NewInMemoryThreadStore creates a new in-memory thread store.
func NewInMemoryThreadStore() *InMemoryThreadStore {
	return &InMemoryThreadStore{
		threads:  make(map[string]*Thread),
		events:   make(map[string][]json.RawMessage),
		sessions: make(map[string]*SessionRecord),
	}
}

func (s *InMemoryThreadStore) CreateThread(title string) (*Thread, error) {
	t := &Thread{
		ID:        uuid.NewString(),
		Title:     title,
		CreatedAt: time.Now().UnixMilli(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threads[t.ID] = t
	return t, nil
}

func (s *InMemoryThreadStore) ListThreads() ([]*Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Thread, 0, len(s.threads))
	for _, t := range s.threads {
		if t.DeletedAt == nil {
			result = append(result, t)
		}
	}
	return result, nil
}

func (s *InMemoryThreadStore) DeleteThread(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[id]
	if !ok {
		return fmt.Errorf("thread %q not found", id)
	}
	now := time.Now().UnixMilli()
	t.DeletedAt = &now
	return nil
}

func (s *InMemoryThreadStore) UpdateTitle(id string, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.threads[id]
	if !ok {
		return fmt.Errorf("thread %q not found", id)
	}
	t.Title = title
	return nil
}

func (s *InMemoryThreadStore) AddEvent(threadID string, event json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[threadID] = append(s.events[threadID], event)
	return nil
}

func (s *InMemoryThreadStore) GetEvents(threadID string) ([]json.RawMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.events[threadID]
	result := make([]json.RawMessage, len(events))
	copy(result, events)
	return result, nil
}

func (s *InMemoryThreadStore) CreateSession(threadID, breedID string) (*SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Count existing sessions for this thread to compute seq
	seq := 0
	for _, sess := range s.sessions {
		if sess.ThreadID == threadID {
			seq++
		}
	}
	sess := &SessionRecord{
		ID:        uuid.NewString(),
		ThreadID:  threadID,
		BreedID:   breedID,
		Seq:       seq + 1,
		Status:    "active",
		CreatedAt: time.Now().UnixMilli(),
	}
	s.sessions[sess.ID] = sess
	return sess, nil
}

func (s *InMemoryThreadStore) ListSessions(threadID string) ([]*SessionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*SessionRecord
	for _, sess := range s.sessions {
		if sess.ThreadID == threadID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *InMemoryThreadStore) UnsealSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	sess.Status = "active"
	sess.SealedAt = nil
	sess.SealReason = ""
	return nil
}
