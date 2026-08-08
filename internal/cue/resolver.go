package cue

import (
	"context"
	"time"
)

// ResolvedCue is the result of a lane resolver: the content to inject.
type ResolvedCue struct {
	EnvelopeID  string `json:"envelope_id"`
	Lane        string `json:"lane"`
	Content     string `json:"content"`     // the actual memory content to inject
	SourceID    string `json:"source_id"`   // ID of the source memory entry
	IsRecalled  bool   `json:"is_recalled"` // true if content was retrieved from storage
}

// LaneResolver resolves recall opportunities for a specific lane into content.
// Each resolver selects deterministic or bounded retrieval — no global cross-lane score.
type LaneResolver interface {
	// Lane returns the lane this resolver handles.
	Lane() string
	// Resolve retrieves content for the given subject and reason.
	// Returns zero results on error (fail-closed, graceful degradation).
	Resolve(ctx context.Context, subject, reason string, budget int) ([]ResolvedCue, error)
}

// LaneResolverRegistry holds resolvers for all lanes.
// There is NO global cross-lane score — each resolver is independent.
type LaneResolverRegistry struct {
	resolvers map[string]LaneResolver
}

// NewResolverRegistry creates a new LaneResolverRegistry.
func NewResolverRegistry() *LaneResolverRegistry {
	return &LaneResolverRegistry{resolvers: make(map[string]LaneResolver)}
}

// Register adds a resolver for a lane.
func (r *LaneResolverRegistry) Register(resolver LaneResolver) {
	r.resolvers[resolver.Lane()] = resolver
}

// Resolve resolves recall opportunities for a given lane.
// Returns empty slice if no resolver is registered (fail-closed).
func (r *LaneResolverRegistry) Resolve(ctx context.Context, lane, subject, reason string, budget int) ([]ResolvedCue, error) {
	resolver, ok := r.resolvers[lane]
	if !ok {
		return nil, nil // fail-closed: no resolver = no cues
	}
	result, err := resolver.Resolve(ctx, subject, reason, budget)
	if err != nil {
		return nil, nil // fail-closed: error = no cues
	}
	return result, nil
}

// HasResolver checks if a resolver is registered for a lane.
func (r *LaneResolverRegistry) HasResolver(lane string) bool {
	_, ok := r.resolvers[lane]
	return ok
}

// RegisteredLanes returns all lanes with registered resolvers.
func (r *LaneResolverRegistry) RegisteredLanes() []string {
	result := make([]string, 0, len(r.resolvers))
	for lane := range r.resolvers {
		result = append(result, lane)
	}
	return result
}

// --- Built-in resolvers ---

// StaticResolver is a simple resolver that returns pre-configured content.
// Useful for testing and for lanes with static knowledge.
type StaticResolver struct {
	lane    string
	entries []StaticEntry
}

// StaticEntry is a pre-configured memory entry.
type StaticEntry struct {
	ID      string
	Content string
}

// NewStaticResolver creates a resolver with static content.
func NewStaticResolver(lane string, entries []StaticEntry) *StaticResolver {
	return &StaticResolver{lane: lane, entries: entries}
}

func (r *StaticResolver) Lane() string { return r.lane }

func (r *StaticResolver) Resolve(ctx context.Context, subject, reason string, budget int) ([]ResolvedCue, error) {
	var result []ResolvedCue
	for _, e := range r.entries {
		result = append(result, ResolvedCue{
			Lane:       r.lane,
			Content:    e.Content,
			SourceID:   e.ID,
			IsRecalled: true,
		})
	}
	return result, nil
}

// TimeoutResolver wraps a resolver with a timeout for bounded retrieval.
type TimeoutResolver struct {
	inner    LaneResolver
	timeout  time.Duration
}

// NewTimeoutResolver wraps a resolver with a timeout.
func NewTimeoutResolver(inner LaneResolver, timeout time.Duration) *TimeoutResolver {
	return &TimeoutResolver{inner: inner, timeout: timeout}
}

func (r *TimeoutResolver) Lane() string { return r.inner.Lane() }

func (r *TimeoutResolver) Resolve(ctx context.Context, subject, reason string, budget int) ([]ResolvedCue, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.inner.Resolve(ctx, subject, reason, budget)
}
