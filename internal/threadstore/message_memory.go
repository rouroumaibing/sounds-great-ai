package threadstore

import (
	"fmt"
	"sync"
	"time"
)

// memoryMessageStore is an in-memory implementation of MessageStore.
// Thread-safe via sync.RWMutex.
type memoryMessageStore struct {
	mu       sync.RWMutex
	messages map[string][]*Message // threadID → messages (chronological order)
	nextID   int64
}

// NewMemoryMessageStore creates a new in-memory MessageStore.
func NewMemoryMessageStore() MessageStore {
	return &memoryMessageStore{
		messages: make(map[string][]*Message),
	}
}

func (s *memoryMessageStore) Append(msg *Message) error {
	if msg.ThreadID == "" {
		return fmt.Errorf("threadID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", s.nextID)
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	s.messages[msg.ThreadID] = append(s.messages[msg.ThreadID], msg)
	return nil
}

func (s *memoryMessageStore) GetByThread(threadID string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.messages[threadID]
	if len(msgs) == 0 {
		return nil, nil
	}

	// Return most recent N messages in chronological order (oldest → newest)
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}

	// Return a copy to avoid external mutation
	result := make([]*Message, len(msgs))
	copy(result, msgs)
	return result, nil
}

func (s *memoryMessageStore) GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.messages[threadID]
	if len(msgs) == 0 {
		return nil, nil
	}

	// Filter messages older than (before, beforeID)
	var filtered []*Message
	for _, m := range msgs {
		if !before.IsZero() {
			if m.Timestamp.After(before) {
				continue
			}
			if m.Timestamp.Equal(before) && m.ID >= beforeID {
				continue
			}
		}
		filtered = append(filtered, m)
	}

	// Take the most recent `limit` from filtered, then return in ascending order
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	result := make([]*Message, len(filtered))
	copy(result, filtered)
	return result, nil
}
