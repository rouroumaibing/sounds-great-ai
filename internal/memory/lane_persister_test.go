package memory

import (
	"path/filepath"
	"testing"
)

// TestLaneRegistryPersistRoundTrip verifies that submitted/approved entries
// survive a registry teardown + reopen (the core P0 guarantee: typed memory
// is not lost on restart).
func TestLaneRegistryPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lanes.json")

	reg := NewLaneRegistryAt(path)
	decision := reg.Lane(LaneDecision)
	if decision == nil {
		t.Fatal("decision lane is nil")
	}
	e := decision.Submit("decided to use Go for platform", "session:s1")
	if !decision.Approve(e.ID) {
		t.Fatal("approve failed")
	}
	taste := reg.Lane(LaneTaste)
	taste.Submit("prefer dark mode", "session:s1")
	reg.Close()

	// Reopen from disk.
	reopened := NewLaneRegistryAt(path)
	defer reopened.Close()

	truth := reopened.Lane(LaneDecision).Truth()
	if len(truth) != 1 {
		t.Fatalf("expected 1 truth after reopen, got %d", len(truth))
	}
	if truth[0].Content != "decided to use Go for platform" {
		t.Errorf("wrong truth content after reopen: %s", truth[0].Content)
	}
	if truth[0].Status != StatusApproved {
		t.Errorf("expected approved status, got %s", truth[0].Status)
	}

	pending := reopened.Lane(LaneTaste).Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending after reopen, got %d", len(pending))
	}
	if pending[0].Content != "prefer dark mode" {
		t.Errorf("wrong pending content after reopen: %s", pending[0].Content)
	}
}

// TestLaneRegistryPersistRetireForget verifies retired/forgotten entries keep
// their status across a reopen (lifecycle truth must not reset to pending).
func TestLaneRegistryPersistRetireForget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lanes.json")

	reg := NewLaneRegistryAt(path)
	l := reg.Lane(LaneProfile)
	e1 := l.Submit("I am a backend developer", "session:s1")
	l.Approve(e1.ID)
	l.Retire(e1.ID)
	e2 := l.Submit("I prefer Go", "session:s1")
	l.Reject(e2.ID)
	reg.Close()

	reopened := NewLaneRegistryAt(path)
	defer reopened.Close()
	got1, _ := reopened.Lane(LaneProfile).Get(e1.ID)
	if got1.Status != StatusRetired {
		t.Errorf("expected retired after reopen, got %s", got1.Status)
	}
	got2, _ := reopened.Lane(LaneProfile).Get(e2.ID)
	if got2.Status != StatusForgotten {
		t.Errorf("expected forgotten after reopen, got %s", got2.Status)
	}
	if len(reopened.Lane(LaneProfile).Truth()) != 0 {
		t.Error("no entry should be truth after reopen")
	}
}
