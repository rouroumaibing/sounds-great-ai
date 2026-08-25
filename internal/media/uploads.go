// Package media implements the rich-media layer: the uploads store is the
// source of truth for generated/attached resources, rich blocks archive
// structured content, and a promoter lifts staged generative artifacts into
// the uploads location so they become addressable (roadmap P1-B).
package media

import (
	"errors"
	"sync"
	"time"
)

// ResourceKind classifies an uploaded resource.
type ResourceKind string

const (
	// KindImage is a still image.
	KindImage ResourceKind = "image"
	// KindAudio is an audio clip.
	KindAudio ResourceKind = "audio"
	// KindVideo is a video clip.
	KindVideo ResourceKind = "video"
	// KindFile is a generic file.
	KindFile ResourceKind = "file"
)

// Resource is the canonical metadata record for an uploaded artifact. The Path
// is the canonical location under the uploads root (e.g. "/uploads/<id>").
type Resource struct {
	ID        string
	Kind      ResourceKind
	Path      string
	Size      int64
	CreatedAt time.Time
	// Source records provenance (e.g. "tts", "generation:image", "user").
	Source string
}

// Store is the uploads source of truth. Goroutine-safe implementations only.
type Store interface {
	Put(r Resource) error
	Get(id string) (Resource, error)
	Delete(id string) error
	List(kind ResourceKind) []Resource
}

// ErrResourceNotFound is returned when an id is unknown.
var ErrResourceNotFound = errors.New("media: resource not found")

// InMemoryStore is an in-process Store for tests and single-process use. A
// production store would persist to the uploads directory with atomic writes.
type InMemoryStore struct {
	mu sync.Mutex
	m  map[string]Resource
}

// NewInMemoryStore creates an empty store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{m: make(map[string]Resource)}
}

// Put records a resource (overwrites on id collision).
func (s *InMemoryStore) Put(r Resource) error {
	if r.ID == "" {
		return errors.New("media: empty resource id")
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[r.ID] = r
	return nil
}

// Get returns a resource by id.
func (s *InMemoryStore) Get(id string) (Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.m[id]
	if !ok {
		return Resource{}, ErrResourceNotFound
	}
	return r, nil
}

// Delete removes a resource.
func (s *InMemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[id]; !ok {
		return ErrResourceNotFound
	}
	delete(s.m, id)
	return nil
}

// List returns resources of a kind (or all when kind == "").
func (s *InMemoryStore) List(kind ResourceKind) []Resource {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Resource
	for _, r := range s.m {
		if kind == "" || r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}
