package telemetry

import (
	"sync"
	"time"
)

// Span is a span snapshot stored in TraceStore (already redacted).
type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]interface{}
	Status     string // "ok" | "error"
}

// TraceStore is an in-memory ring buffer span store.
type TraceStore struct {
	spans   []Span
	maxSize int
	mu      sync.RWMutex
}

// TraceStoreStats describes store statistics.
type TraceStoreStats struct {
	Count   int
	MaxSize int
	Oldest  time.Time
}

// NewTraceStore creates a TraceStore with the given capacity.
func NewTraceStore(maxSize int) *TraceStore {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &TraceStore{spans: make([]Span, 0, maxSize), maxSize: maxSize}
}

// Add adds a span to the ring buffer. Evicts the oldest entry when full.
func (ts *TraceStore) Add(span Span) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if len(ts.spans) >= ts.maxSize {
		ts.spans = ts.spans[1:] // remove oldest
	}
	ts.spans = append(ts.spans, span)
}

// Query returns spans matching the filters. Empty traceID/breedID means no filter.
// limit<=0 defaults to 100, capped at 1000. Results are newest-first.
func (ts *TraceStore) Query(traceID, breedID string, limit int) []Span {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]Span, 0, limit)
	for i := len(ts.spans) - 1; i >= 0 && len(result) < limit; i-- {
		s := ts.spans[i]
		if traceID != "" && s.TraceID != traceID {
			continue
		}
		if breedID != "" {
			if v, ok := s.Attributes["breed"].(string); !ok || v != breedID {
				continue
			}
		}
		result = append(result, s)
	}
	return result
}

// Stats returns store statistics.
func (ts *TraceStore) Stats() TraceStoreStats {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	stats := TraceStoreStats{Count: len(ts.spans), MaxSize: ts.maxSize}
	if len(ts.spans) > 0 {
		stats.Oldest = ts.spans[0].StartTime
	}
	return stats
}
