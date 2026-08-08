package cue

import "testing"

func TestConsumptionLedgerRecordPresented(t *testing.T) {
	cl := NewConsumptionLedger()
	e := cl.RecordPresented("env-1", "person", "session-1")
	if e.Action != EpisodePresented {
		t.Errorf("expected presented, got %s", e.Action)
	}
	if cl.EpisodeCount() != 1 {
		t.Errorf("expected 1 episode, got %d", cl.EpisodeCount())
	}
}

func TestConsumptionLedgerAllActions(t *testing.T) {
	cl := NewConsumptionLedger()
	cl.RecordPresented("env-1", "person", "session-1")
	cl.RecordDrilled("env-1", "person", "session-1")
	cl.RecordApplied("env-1", "person", "session-1")
	cl.RecordDismissed("env-2", "taste", "session-1")
	if cl.EpisodeCount() != 4 {
		t.Fatalf("expected 4 episodes, got %d", cl.EpisodeCount())
	}
}

func TestConsumptionLedgerSourceInvalidation(t *testing.T) {
	cl := NewConsumptionLedger()
	if cl.IsSourceInvalidated("src-1") {
		t.Error("source should not be invalidated initially")
	}
	cl.InvalidateSource("src-1", "corrected")
	if !cl.IsSourceInvalidated("src-1") {
		t.Error("source should be invalidated after InvalidateSource")
	}
	if cl.IsSourceInvalidated("src-2") {
		t.Error("src-2 should not be invalidated")
	}
}

func TestConsumptionLedgerEpisodesForEnvelope(t *testing.T) {
	cl := NewConsumptionLedger()
	cl.RecordPresented("env-1", "person", "session-1")
	cl.RecordDrilled("env-1", "person", "session-1")
	cl.RecordPresented("env-2", "taste", "session-1")
	episodes := cl.EpisodesForEnvelope("env-1")
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes for env-1, got %d", len(episodes))
	}
}

func TestConsumptionLedgerEpisodesForSession(t *testing.T) {
	cl := NewConsumptionLedger()
	cl.RecordPresented("env-1", "person", "session-1")
	cl.RecordPresented("env-2", "taste", "session-2")
	s1 := cl.EpisodesForSession("session-1")
	s2 := cl.EpisodesForSession("session-2")
	if len(s1) != 1 {
		t.Fatalf("expected 1 episode for session-1, got %d", len(s1))
	}
	if len(s2) != 1 {
		t.Fatalf("expected 1 episode for session-2, got %d", len(s2))
	}
}

func TestConsumptionLedgerAllInvalidations(t *testing.T) {
	cl := NewConsumptionLedger()
	cl.InvalidateSource("src-1", "corrected")
	cl.InvalidateSource("src-2", "forgotten")
	invs := cl.AllInvalidations()
	if len(invs) != 2 {
		t.Fatalf("expected 2 invalidations, got %d", len(invs))
	}
}
