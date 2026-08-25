package frontdesk

import "testing"

func TestTriage_RoutesByIntent(t *testing.T) {
	if r := Triage(Request{Query: "recall my memory about X"}, 0.9, 0.5); r.Capability != "memory" {
		t.Fatalf("memory query should route to memory, got %s", r.Capability)
	}
	if r := Triage(Request{Query: "what is the feature roadmap?"}, 0.9, 0.5); r.Capability != "feature" {
		t.Fatalf("feature query should route to feature, got %s", r.Capability)
	}
}

func TestTriage_FailClosedLowConfidence(t *testing.T) {
	// Even a memory-looking query is sent to triage when confidence is low.
	r := Triage(Request{Query: "remember this"}, 0.2, 0.5)
	if r.Capability != "triage" {
		t.Fatalf("low-confidence must fail closed to triage, got %s", r.Capability)
	}
}
