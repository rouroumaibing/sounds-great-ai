// Package observability provides cross-channel friction-signal aggregation
// (F245): dedup, cluster, and rollup of friction reports from many channels into
// a single source-of-truth view.
package observability

import (
	"sort"
	"sync"
)

// FrictionSignal is a single friction report from any channel.
type FrictionSignal struct {
	ID      string
	Channel string // e.g. "cli" | "ui" | "webhook"
	Kind    string // e.g. "stall" | "error" | "retry"
	Key     string // dedup key (e.g. "stall:thread-12"); empty falls back to ID
	Weight  int
}

// ChannelRollup is the aggregated friction for one channel.
type ChannelRollup struct {
	Channel string
	Count   int
	Weight  int
}

// FrictionAggregator dedups, clusters, and rolls up friction signals across
// channels (F245). It is the single source of truth for the aggregated view.
type FrictionAggregator struct {
	mu    sync.Mutex
	byKey map[string]*FrictionSignal
}

// NewFrictionAggregator creates an empty aggregator.
func NewFrictionAggregator() *FrictionAggregator {
	return &FrictionAggregator{byKey: make(map[string]*FrictionSignal)}
}

// Ingest dedups by Key: only the first signal per Key is kept; subsequent
// duplicates (even from another channel) are ignored. Returns true if accepted.
func (a *FrictionAggregator) Ingest(s FrictionSignal) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := s.Key
	if key == "" {
		key = s.ID
	}
	if _, ok := a.byKey[key]; ok {
		return false // duplicate, deduped
	}
	cp := s
	cp.Key = key
	a.byKey[key] = &cp
	return true
}

// Rollup returns per-channel rollups (count + weight), sorted by channel.
func (a *FrictionAggregator) Rollup() []ChannelRollup {
	a.mu.Lock()
	defer a.mu.Unlock()
	m := make(map[string]*ChannelRollup)
	for _, s := range a.byKey {
		r := m[s.Channel]
		if r == nil {
			r = &ChannelRollup{Channel: s.Channel}
			m[s.Channel] = r
		}
		r.Count++
		r.Weight += s.Weight
	}
	out := make([]ChannelRollup, 0, len(m))
	for _, r := range m {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Channel < out[j].Channel })
	return out
}
