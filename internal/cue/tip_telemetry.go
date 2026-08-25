package cue

import "sync"

// TipTelemetry aggregates capability-tip consumption telemetry server-side
// (F268): presented/drilled/applied/dismissed counts per envelope, plus a
// usefulness ratio (applied / (presented - dismissed)).
type TipTelemetry struct {
	mu        sync.Mutex
	presented int
	drilled   int
	applied   int
	dismissed int
}

// NewTipTelemetry creates an empty telemetry aggregator.
func NewTipTelemetry() *TipTelemetry { return &TipTelemetry{} }

// Observe records one consumption-episode action (F268 server-side aggregation).
func (t *TipTelemetry) Observe(action ConsumptionEpisodeAction) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch action {
	case EpisodePresented:
		t.presented++
	case EpisodeDrilled:
		t.drilled++
	case EpisodeApplied:
		t.applied++
	case EpisodeDismissed:
		t.dismissed++
	}
}

// Usefulness returns applied / (presented - dismissed) in [0,1]. When no tips
// were meaningfully presented it returns 0 (fail-closed: never report false
// usefulness).
func (t *TipTelemetry) Usefulness() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	meaningful := t.presented - t.dismissed
	if meaningful <= 0 {
		return 0
	}
	u := float64(t.applied) / float64(meaningful)
	if u > 1 {
		u = 1
	}
	return u
}

// Counts returns the raw aggregated counts.
func (t *TipTelemetry) Counts() (presented, drilled, applied, dismissed int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.presented, t.drilled, t.applied, t.dismissed
}
