package stores

import (
	"encoding/json"

	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	"sounds-great-ai/internal/threadstore"
)

// ThreadStoreAdapter wraps an existing threadstore.ThreadStore to implement
// the domain ports.IThreadStore interface.
type ThreadStoreAdapter struct {
	inner threadstore.ThreadStore
}

// NewThreadStoreAdapter creates a new ThreadStoreAdapter.
func NewThreadStoreAdapter(inner threadstore.ThreadStore) *ThreadStoreAdapter {
	return &ThreadStoreAdapter{inner: inner}
}

func (a *ThreadStoreAdapter) CreateThread(title string) (*threadPorts.Thread, error) {
	t, err := a.inner.CreateThread(title)
	if err != nil {
		return nil, err
	}
	return &threadPorts.Thread{
		ID:        t.ID,
		Title:     t.Title,
		CreatedAt: t.CreatedAt,
		DeletedAt: t.DeletedAt,
	}, nil
}

func (a *ThreadStoreAdapter) ListThreads() ([]*threadPorts.Thread, error) {
	threads, err := a.inner.ListThreads()
	if err != nil {
		return nil, err
	}
	result := make([]*threadPorts.Thread, len(threads))
	for i, t := range threads {
		result[i] = &threadPorts.Thread{
			ID:        t.ID,
			Title:     t.Title,
			CreatedAt: t.CreatedAt,
			DeletedAt: t.DeletedAt,
		}
	}
	return result, nil
}

func (a *ThreadStoreAdapter) DeleteThread(id string) error {
	return a.inner.DeleteThread(id)
}

func (a *ThreadStoreAdapter) UpdateTitle(id string, title string) error {
	return a.inner.UpdateTitle(id, title)
}

func (a *ThreadStoreAdapter) AddEvent(threadID string, event json.RawMessage) error {
	return a.inner.AddEvent(threadID, event)
}

func (a *ThreadStoreAdapter) GetEvents(threadID string) ([]json.RawMessage, error) {
	return a.inner.GetEvents(threadID)
}

func (a *ThreadStoreAdapter) CreateSession(threadID, breedID string) (*threadPorts.SessionRecord, error) {
	s, err := a.inner.CreateSession(threadID, breedID)
	if err != nil {
		return nil, err
	}
	return &threadPorts.SessionRecord{
		ID:           s.ID,
		ThreadID:     s.ThreadID,
		BreedID:      s.BreedID,
		Seq:          s.Seq,
		Status:       s.Status,
		MessageCount: s.MessageCount,
		SealReason:   s.SealReason,
		CreatedAt:    s.CreatedAt,
		SealedAt:     s.SealedAt,
	}, nil
}

func (a *ThreadStoreAdapter) ListSessions(threadID string) ([]*threadPorts.SessionRecord, error) {
	sessions, err := a.inner.ListSessions(threadID)
	if err != nil {
		return nil, err
	}
	result := make([]*threadPorts.SessionRecord, len(sessions))
	for i, s := range sessions {
		result[i] = &threadPorts.SessionRecord{
			ID:           s.ID,
			ThreadID:     s.ThreadID,
			BreedID:      s.BreedID,
			Seq:          s.Seq,
			Status:       s.Status,
			MessageCount: s.MessageCount,
			SealReason:   s.SealReason,
			CreatedAt:    s.CreatedAt,
			SealedAt:     s.SealedAt,
		}
	}
	return result, nil
}

func (a *ThreadStoreAdapter) UnsealSession(sessionID string) error {
	return a.inner.UnsealSession(sessionID)
}
