package memory

import "testing"

func TestDispositionAccept(t *testing.T) {
	reg := NewLaneRegistry()
	dr := NewDispositionRecorder()
	lane := reg.Lane(LaneDecision)
	e := lane.Submit("decided to use Go", "session:s1")

	d, err := dr.Record(reg, e.ID, LaneDecision, DispositionAccept, "", "user-1")
	if err != nil {
		t.Fatalf("record error: %v", err)
	}
	if d.Action != DispositionAccept {
		t.Errorf("expected accept, got %s", d.Action)
	}
	entry, _ := lane.Get(e.ID)
	if entry.Status != StatusApproved {
		t.Errorf("expected approved, got %s", entry.Status)
	}
}

func TestDispositionReject(t *testing.T) {
	reg := NewLaneRegistry()
	dr := NewDispositionRecorder()
	lane := reg.Lane(LaneTaste)
	e := lane.Submit("prefer dark mode", "session:s1")

	_, err := dr.Record(reg, e.ID, LaneTaste, DispositionReject, "", "user-1")
	if err != nil {
		t.Fatalf("record error: %v", err)
	}
	entry, _ := lane.Get(e.ID)
	if entry.Status != StatusForgotten {
		t.Errorf("expected forgotten, got %s", entry.Status)
	}
}

func TestDispositionModify(t *testing.T) {
	reg := NewLaneRegistry()
	dr := NewDispositionRecorder()
	lane := reg.Lane(LaneProfile)
	e := lane.Submit("I am a developer", "session:s1")

	_, err := dr.Record(reg, e.ID, LaneProfile, DispositionModify, "I am a senior Go developer", "user-1")
	if err != nil {
		t.Fatalf("record error: %v", err)
	}
	entry, _ := lane.Get(e.ID)
	if entry.Status != StatusApproved {
		t.Errorf("expected approved, got %s", entry.Status)
	}
	if entry.Content != "I am a senior Go developer" {
		t.Errorf("expected modified content, got %s", entry.Content)
	}
}

func TestDispositionLaneNotFound(t *testing.T) {
	reg := NewLaneRegistry()
	dr := NewDispositionRecorder()
	_, err := dr.Record(reg, "x", "unknown-lane", DispositionAccept, "", "user-1")
	if err == nil {
		t.Fatal("expected error for unknown lane")
	}
}

func TestConsumptionTracker(t *testing.T) {
	ct := NewConsumptionTracker()
	e1 := ct.Record("entry-1", LaneDecision, ConsumptionPresented, "session-1")
	e2 := ct.Record("entry-1", LaneDecision, ConsumptionApplied, "session-1")
	e3 := ct.Record("entry-2", LaneTaste, ConsumptionDismissed, "session-1")

	if ct.Count() != 3 {
		t.Fatalf("expected 3 events, got %d", ct.Count())
	}
	forEntry1 := ct.ForEntry("entry-1")
	if len(forEntry1) != 2 {
		t.Fatalf("expected 2 events for entry-1, got %d", len(forEntry1))
	}
	if e1.Action != ConsumptionPresented {
		t.Errorf("expected presented, got %s", e1.Action)
	}
	if e2.Action != ConsumptionApplied {
		t.Errorf("expected applied, got %s", e2.Action)
	}
	if e3.Action != ConsumptionDismissed {
		t.Errorf("expected dismissed, got %s", e3.Action)
	}
}

func TestLifecycleTrace(t *testing.T) {
	lt := NewLifecycleTrace()
	lt.RecordCreation("entry-1", LaneDecision, "submitted from session-1")
	lt.RecordConsumption("entry-1", LaneDecision, "presented in session-2")
	lt.RecordCorrection("entry-1", LaneDecision, "modified by user")

	records := lt.ForEntry("entry-1")
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].Axis != AxisCreation {
		t.Errorf("expected creation axis, got %s", records[0].Axis)
	}
	if records[1].Axis != AxisConsumption {
		t.Errorf("expected consumption axis, got %s", records[1].Axis)
	}
	if records[2].Axis != AxisCorrection {
		t.Errorf("expected correction axis, got %s", records[2].Axis)
	}
}
