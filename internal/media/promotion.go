package media

import (
	"errors"
	"time"
)

// GeneratedArtifact is a staged generative output (e.g. an image rendered by a
// model) that is not yet part of the uploads source of truth.
type GeneratedArtifact struct {
	ID          string
	Kind        ResourceKind
	StagingPath string // where the bytes currently live (staging)
	Size        int64
	Source      string // provenance label, e.g. "generation:image"
}

// Promoter lifts staged artifacts into the uploads Store, making them
// addressable resources. It is the "artifact promotion" path (roadmap P1-B):
// a generated product goes through promotion before it can be referenced by a
// rich block or delivered to a user.
type Promoter struct {
	store Store
	// Root is the canonical uploads root (default "/uploads"). Production code
	// should inject the configured root; never hardcode per AGENTS.md.
	Root string
}

// NewPromoter creates a promoter over the given store.
func NewPromoter(store Store) *Promoter {
	return &Promoter{store: store, Root: "/uploads"}
}

// Promote writes the artifact into the uploads Store and returns the canonical
// Resource. It fails closed on a missing store or empty id and never invents a
// path outside the configured root.
func (p *Promoter) Promote(a GeneratedArtifact) (Resource, error) {
	if p.store == nil {
		return Resource{}, errors.New("media: promoter has no store")
	}
	if a.ID == "" {
		return Resource{}, errors.New("media: empty artifact id")
	}
	res := Resource{
		ID:        a.ID,
		Kind:      a.Kind,
		Path:      p.Root + "/" + a.ID,
		Size:      a.Size,
		CreatedAt: time.Now(),
		Source:    a.Source,
	}
	if err := p.store.Put(res); err != nil {
		return Resource{}, err
	}
	return res, nil
}
