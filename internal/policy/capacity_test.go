package policy

import "testing"

func TestCapacity_UsageRatioAndExceeds(t *testing.T) {
	s := CapacitySnapshot{Used: 50, Limit: 100}
	if got := s.UsageRatio(); got != 0.5 {
		t.Fatalf("ratio = %v, want 0.5", got)
	}
	if s.Exceeds() {
		t.Fatal("50/100 should not exceed")
	}
	if !(CapacitySnapshot{Used: 100, Limit: 100}.Exceeds()) {
		t.Fatal("100/100 must exceed")
	}
}

func TestCapacity_UngovernedRatioZero(t *testing.T) {
	if (CapacitySnapshot{Used: 999, Limit: 0}).UsageRatio() != 0 {
		t.Fatal("ungoverned limit must report zero ratio")
	}
}

func TestEvaluateSealGate_Exceeds(t *testing.T) {
	seal, w, err := EvaluateSealGate(CapacitySnapshot{Used: 100, Limit: 100}, 0.9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !seal || w.Level != WarnCritical || !w.ShouldSeal {
		t.Fatalf("at-limit must seal critically, got seal=%v warn=%+v", seal, w)
	}
}

func TestEvaluateSealGate_Threshold(t *testing.T) {
	seal, w, err := EvaluateSealGate(CapacitySnapshot{Used: 92, Limit: 100}, 0.9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !seal || w.Level != WarnCritical {
		t.Fatalf("92%% at 0.9 threshold must seal, got seal=%v warn=%+v", seal, w)
	}
}

func TestEvaluateSealGate_PinnedBypassesSeal(t *testing.T) {
	seal, w, err := EvaluateSealGate(CapacitySnapshot{Used: 92, Limit: 100, Pinned: true}, 0.9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seal {
		t.Fatal("pinned session must not be sealed even at threshold")
	}
	if w.Level != WarnCritical {
		t.Fatalf("pinned still emits critical warning, got %+v", w)
	}
}

func TestEvaluateSealGate_Warning(t *testing.T) {
	_, w, err := EvaluateSealGate(CapacitySnapshot{Used: 75, Limit: 100}, 0.9)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if w.Level != WarnWarning {
		t.Fatalf("75%% should warn, got %+v", w)
	}
}

func TestEvaluateSealGate_UngovernedFailClosed(t *testing.T) {
	seal, w, err := EvaluateSealGate(CapacitySnapshot{Used: 0, Limit: 0}, 0.9)
	if err == nil {
		t.Fatal("ungoverned capacity must error (fail-closed)")
	}
	if !seal || !w.ShouldSeal {
		t.Fatal("ungoverned must imply seal now")
	}
}

func TestSessionPin_Exempts(t *testing.T) {
	s := CapacitySnapshot{Used: 92, Limit: 100}.Pin(SessionPin{SessionID: "s1", Principal: "u1"})
	if !s.Pinned {
		t.Fatal("Pin must set Pinned=true")
	}
}
