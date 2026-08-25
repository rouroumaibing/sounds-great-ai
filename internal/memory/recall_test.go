package memory

import (
	"path/filepath"
	"testing"
)

// F200 + F263: RecallStore records a recall event, persists it, and applies the
// three-axis semantic classification + maturity defaults from the operator
// outcome (the consumption-verification loop).
func TestRecallStore_RecordAndThreeAxisDefaults(t *testing.T) {
	dir := t.TempDir()
	s := NewRecallStore(filepath.Join(dir, "mem"))
	defer s.Close()

	ev := &RecallEvent{
		OperatorID: "op-1",
		Kind:       "push",
		Trigger:    "session_bootstrap",
		EntryIDs:   []string{"e1", "e2"},
		Outcome:    RecallOutcomeUsed,
	}
	s.Record(ev)

	if ev.ID == "" {
		t.Fatal("Record must fill ID")
	}
	if ev.Count != 2 {
		t.Fatalf("Record must set Count from EntryIDs, got %d", ev.Count)
	}
	// F263: axis/maturity default from outcome → used ⇒ beneficial/measured.
	if ev.Axis != AxisBeneficial {
		t.Fatalf("expected axis=beneficial, got %q", ev.Axis)
	}
	if ev.Maturity != MaturityMeasured {
		t.Fatalf("expected maturity=measured, got %q", ev.Maturity)
	}

	recent := s.Recent(10)
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent event, got %d", len(recent))
	}
	if recent[0].ID != ev.ID {
		t.Fatal("Recent must return the recorded event")
	}
}

// F200: Ledger reports the three-axis breakdown with maturity labels per window.
func TestRecallStore_LedgerThreeAxis(t *testing.T) {
	dir := t.TempDir()
	s := NewRecallStore(filepath.Join(dir, "mem"))
	defer s.Close()

	// Two used (beneficial) + one ignored (attention) + one unverified (unmet).
	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeUsed})
	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeUsed})
	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeIgnored})
	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeEmpty})

	ledger := s.Ledger([]int{7})
	stat, ok := ledger["7d"]
	if !ok {
		t.Fatal("ledger must contain 7d window")
	}
	if stat.Total != 4 {
		t.Fatalf("expected total=4, got %d", stat.Total)
	}
	if stat.Used != 2 {
		t.Fatalf("expected used=2, got %d", stat.Used)
	}
	if stat.Ignored != 1 {
		t.Fatalf("expected ignored=1, got %d", stat.Ignored)
	}
	if stat.Unverified != 1 {
		t.Fatalf("expected unverified=1, got %d", stat.Unverified)
	}
	// F263 three-axis semantic breakdown.
	if stat.Beneficial != 2 {
		t.Fatalf("expected beneficial=2, got %d", stat.Beneficial)
	}
	if stat.Attention != 1 {
		t.Fatalf("expected attention=1, got %d", stat.Attention)
	}
	if stat.Unmet != 1 {
		t.Fatalf("expected unmet=1, got %d", stat.Unmet)
	}
	// Maturity histogram: 3 measured (2 used + 1 ignored) + 1 estimated (unverified).
	if stat.Maturity[MaturityMeasured] != 3 {
		t.Fatalf("expected measured=3, got %d", stat.Maturity[MaturityMeasured])
	}
	if stat.Maturity[MaturityEstimated] != 1 {
		t.Fatalf("expected estimated=1, got %d", stat.Maturity[MaturityEstimated])
	}
}

// F200: MarkOutcome completes the consumption-verification loop and re-affirms
// operator attribution without clobbering it.
func TestRecallStore_MarkOutcomeLoop(t *testing.T) {
	dir := t.TempDir()
	s := NewRecallStore(filepath.Join(dir, "mem"))
	defer s.Close()

	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeEmpty})
	recent := s.Recent(1)
	id := recent[0].ID

	if err := s.MarkOutcome(id, RecallOutcomeIgnored, "", "", "op-2"); err != nil {
		t.Fatal(err)
	}
	after := s.Recent(1)[0]
	if after.Outcome != RecallOutcomeIgnored {
		t.Fatalf("expected ignored, got %q", after.Outcome)
	}
	if after.Axis != AxisAttention {
		t.Fatalf("expected axis=attention, got %q", after.Axis)
	}
	if after.OperatorID != "op-2" {
		t.Fatalf("expected operator reaffirmed to op-2, got %q", after.OperatorID)
	}
}

// F200: invalid outcome is rejected (fail-closed).
func TestRecallStore_MarkOutcomeInvalidRejected(t *testing.T) {
	dir := t.TempDir()
	s := NewRecallStore(filepath.Join(dir, "mem"))
	defer s.Close()
	s.Record(&RecallEvent{OperatorID: "op-1", Outcome: RecallOutcomeEmpty})
	id := s.Recent(1)[0].ID
	if err := s.MarkOutcome(id, "bogus", "", "", ""); err == nil {
		t.Fatal("invalid outcome must be rejected")
	}
}
