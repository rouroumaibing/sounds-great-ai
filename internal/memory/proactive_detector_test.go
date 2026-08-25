package memory

import (
	"testing"
)

// F282 AC-A1: detector is owner-scoped and fail-closed — empty owner produces
// no candidates and panics nowhere.
func TestProactiveDetector_EmptyOwnerFailClosed(t *testing.T) {
	d := NewProactiveDetector("", nil)
	d.Observe("t1", "meeting with Alice about the roadmap")
	if got := d.Candidates(2); len(got) != 0 {
		t.Fatalf("empty owner must produce no candidates, got %d", len(got))
	}
}

// F282 Phase A: a surface recurring across ≥2 distinct threads surfaces as a
// lane-neutral candidate; a one-thread surface does not.
func TestProactiveDetector_CrossThreadCandidate(t *testing.T) {
	d := NewProactiveDetector("op-1", nil)
	// "alice" appears in two distinct threads.
	d.Observe("t1", "meeting with alice about the roadmap")
	d.Observe("t1", "alice also flagged the latency bug")
	d.Observe("t2", "alice suggested a new retention experiment")
	// "bob" appears only once → must NOT be a candidate.
	d.Observe("t3", "bob joined the standup")

	cands := d.Candidates(2)
	if len(cands) != 1 {
		t.Fatalf("expected exactly 1 candidate (alice), got %d: %+v", len(cands), cands)
	}
	c := cands[0]
	if c.Surface != "alice" {
		t.Fatalf("expected surface=alice, got %q", c.Surface)
	}
	if c.ThreadCount != 2 {
		t.Fatalf("expected thread_count=2, got %d", c.ThreadCount)
	}
	if c.MessageCount != 3 {
		t.Fatalf("expected message_count=3, got %d", c.MessageCount)
	}
	if c.OwnerUserID != "op-1" {
		t.Fatalf("expected owner op-1, got %q", c.OwnerUserID)
	}
}

// F282 AC-A3: a dismissed surface must not re-nudge, even if it recurs later.
func TestProactiveDetector_DismissSuppresses(t *testing.T) {
	d := NewProactiveDetector("op-1", nil)
	d.Observe("t1", "carol proposed the zephyr migration")
	d.Observe("t2", "carol reviewed the quartz migration")
	// carol is a cross-thread candidate before dismiss.
	hasCarol := false
	for _, c := range d.Candidates(2) {
		if c.Surface == "carol" {
			hasCarol = true
		}
	}
	if !hasCarol {
		t.Fatal("carol should be a candidate before dismiss")
	}
	d.Dismiss("carol")
	// After dismiss, carol must no longer appear (regardless of other candidates).
	for _, c := range d.Candidates(2) {
		if c.Surface == "carol" {
			t.Fatalf("dismissed surface must not re-nudge, got %+v", c)
		}
	}
	// A later recurrence of carol is still suppressed.
	d.Observe("t3", "carol approved the cobalt migration")
	for _, c := range d.Candidates(2) {
		if c.Surface == "carol" {
			t.Fatalf("dismissed surface must stay suppressed, got %+v", c)
		}
	}
}

// F282 KD-8: known registry surfaces are subtracted from candidacy — detection
// is "not yet recorded", not "unknown".
func TestProactiveDetector_KnownSurfaceSubtracted(t *testing.T) {
	reg := newEventRegistry(t)
	p := reg.Lane(LanePerson).Submit("dave", "session:1")
	p.OperatorID = "op-1"
	reg.Lane(LanePerson).Approve(p.ID)
	er := NewEntityRegistry(reg)
	d := NewProactiveDetector("op-1", er)
	d.Observe("t1", "dave owns the payments lane")
	d.Observe("t2", "dave signed off the release")
	if len(d.Candidates(2)) != 0 {
		t.Fatalf("known surface must be subtracted, got %d candidates", len(d.Candidates(2)))
	}
}

// F282 stopword red line: laughter/particle families must never nudge.
func TestProactiveDetector_StopWordsExcluded(t *testing.T) {
	d := NewProactiveDetector("op-1", nil)
	d.Observe("t1", "哈哈哈")
	d.Observe("t2", "哈哈哈")
	if len(d.Candidates(2)) != 0 {
		t.Fatalf("stopword surface must not be a candidate, got %d", len(d.Candidates(2)))
	}
}

// F282 cross-owner partition: a candidate for op-1 is invisible to op-2.
func TestProactiveDetector_OwnerPartition(t *testing.T) {
	d1 := NewProactiveDetector("op-1", nil)
	d1.Observe("t1", "erin leads the ml platform")
	d1.Observe("t2", "erin owns the feature store")
	// op-2 is a different detector instance; must not see op-1's data.
	d2 := NewProactiveDetector("op-2", nil)
	if len(d2.Candidates(2)) != 0 {
		t.Fatalf("op-2 must not leak op-1 candidates, got %d", len(d2.Candidates(2)))
	}
	if len(d1.Candidates(2)) != 1 {
		t.Fatalf("op-1 should have 1 candidate, got %d", len(d1.Candidates(2)))
	}
}
