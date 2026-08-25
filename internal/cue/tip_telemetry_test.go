package cue

import "testing"

func TestTipTelemetry_Aggregation(t *testing.T) {
	tel := NewTipTelemetry()
	tel.Observe(EpisodePresented)
	tel.Observe(EpisodePresented)
	tel.Observe(EpisodeApplied)
	if p, d, a, dis := tel.Counts(); p != 2 || d != 0 || a != 1 || dis != 0 {
		t.Fatalf("counts wrong: %d %d %d %d", p, d, a, dis)
	}
	// presented=2, dismissed=0 => usefulness = applied/presented = 0.5
	if u := tel.Usefulness(); u != 0.5 {
		t.Fatalf("usefulness should be 0.5, got %v", u)
	}
}

func TestTipTelemetry_FailClosedAllDismissed(t *testing.T) {
	tel := NewTipTelemetry()
	tel.Observe(EpisodePresented)
	tel.Observe(EpisodeDismissed)
	if u := tel.Usefulness(); u != 0 {
		t.Fatalf("all-dismissed must report 0 usefulness, got %v", u)
	}
}
