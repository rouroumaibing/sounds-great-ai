package memory

import "testing"

func TestLaneSubmitAndApprove(t *testing.T) {
	lane := NewLane(LaneDecision)
	e := lane.Submit("decided to use Go for platform", "session:s1")
	if e.Status != StatusPending {
		t.Fatalf("expected pending, got %s", e.Status)
	}
	if !lane.Approve(e.ID) {
		t.Fatal("approve failed")
	}
	got, ok := lane.Get(e.ID)
	if !ok {
		t.Fatal("entry not found after approve")
	}
	if got.Status != StatusApproved {
		t.Errorf("expected approved, got %s", got.Status)
	}
	truth := lane.Truth()
	if len(truth) != 1 {
		t.Fatalf("expected 1 truth, got %d", len(truth))
	}
}

func TestLaneReject(t *testing.T) {
	lane := NewLane(LaneTaste)
	e := lane.Submit("prefer dark mode", "session:s1")
	if !lane.Reject(e.ID) {
		t.Fatal("reject failed")
	}
	got, _ := lane.Get(e.ID)
	if got.Status != StatusForgotten {
		t.Errorf("expected forgotten, got %s", got.Status)
	}
	if len(lane.Truth()) != 0 {
		t.Error("rejected entry should not be truth")
	}
}

func TestLaneRetire(t *testing.T) {
	lane := NewLane(LaneProfile)
	e := lane.Submit("I am a backend developer", "session:s1")
	lane.Approve(e.ID)
	if !lane.Retire(e.ID) {
		t.Fatal("retire failed")
	}
	got, _ := lane.Get(e.ID)
	if got.Status != StatusRetired {
		t.Errorf("expected retired, got %s", got.Status)
	}
	if len(lane.Truth()) != 0 {
		t.Error("retired entry should not be truth")
	}
}

func TestLaneForget(t *testing.T) {
	lane := NewLane(LaneLesson)
	e := lane.Submit("always run tests", "session:s1")
	lane.Approve(e.ID)
	if !lane.Forget(e.ID) {
		t.Fatal("forget failed")
	}
	got, _ := lane.Get(e.ID)
	if got.Status != StatusForgotten {
		t.Errorf("expected forgotten, got %s", got.Status)
	}
}

func TestLaneApproveOnlyPending(t *testing.T) {
	lane := NewLane(LaneEvent)
	e := lane.Submit("deployed v1", "session:s1")
	lane.Approve(e.ID)
	// Cannot approve again
	if lane.Approve(e.ID) {
		t.Error("should not approve non-pending entry")
	}
}

func TestLanePending(t *testing.T) {
	lane := NewLane(LaneEntity)
	e1 := lane.Submit("UserService", "session:s1")
	e2 := lane.Submit("AuthService", "session:s1")
	lane.Approve(e1.ID)
	pending := lane.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != e2.ID {
		t.Error("wrong pending entry")
	}
}

func TestLaneRegistry(t *testing.T) {
	reg := NewLaneRegistry()
	types := reg.LaneTypes()
	if len(types) != 7 {
		t.Fatalf("expected 7 lane types, got %d", len(types))
	}
	for _, lt := range types {
		lane := reg.Lane(lt)
		if lane == nil {
			t.Errorf("lane %s is nil", lt)
		}
	}
}

func TestLaneAll(t *testing.T) {
	lane := NewLane(LanePerson)
	e1 := lane.Submit("Alice is a PM", "session:s1")
	e2 := lane.Submit("Bob is a dev", "session:s1")
	lane.Approve(e1.ID)
	lane.Reject(e2.ID)
	all := lane.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
}
