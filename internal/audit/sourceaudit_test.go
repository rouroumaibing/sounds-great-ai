package audit

import "testing"

func TestSourceAudit_FailClosedEmptySource(t *testing.T) {
	a := SourceAudit{}
	res := a.Check("some claim", "", map[string]bool{"S1": true})
	if res.Passed {
		t.Fatal("empty source must fail closed")
	}
	if len(res.Failed) == 0 || res.Failed[0] != "src:missing" {
		t.Fatalf("expected src:missing failure, got %v", res.Failed)
	}
}

func TestSourceAudit_PassAll(t *testing.T) {
	a := SourceAudit{}
	answers := map[string]bool{"S1": true, "S2": true, "S3": true, "S4": true, "S5": true}
	res := a.Check("claim", "https://example.com/doc#v3", answers)
	if !res.Passed {
		t.Fatalf("all-true answers should pass, failed: %v", res.Failed)
	}
}

func TestSourceAudit_UnansweredFails(t *testing.T) {
	a := SourceAudit{}
	// S3 left unanswered.
	answers := map[string]bool{"S1": true, "S2": true, "S4": true, "S5": true}
	res := a.Check("claim", "src", answers)
	if res.Passed {
		t.Fatal("unanswered check must fail")
	}
	if len(res.Failed) != 1 || res.Failed[0] != "S3" {
		t.Fatalf("expected exactly S3 failed, got %v", res.Failed)
	}
}
