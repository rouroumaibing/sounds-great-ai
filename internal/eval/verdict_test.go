package eval

import (
	"testing"
	"time"
)

func TestValidateVerdictValid(t *testing.T) {
	v := &VerdictHandoffPacket{
		ID:         "v-1",
		DomainID:   "eval:a2a",
		CreatedAt:  time.Now(),
		Phenomenon: "协作延迟上升",
		Verdict:    VerdictFix,
		Evidence: EvidencePacket{
			SnapshotRefs: []string{"snap-1"},
			MetricRefs:   []string{"a2a.latency"},
		},
		RootCause: RootCause{Summary: "路由抖动", Confidence: "high"},
	}
	if err := ValidateVerdict(v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateVerdictEmptyEvidence(t *testing.T) {
	v := &VerdictHandoffPacket{
		ID:         "v-2",
		DomainID:   "eval:a2a",
		Phenomenon: "test",
		Verdict:    VerdictKeepObserve,
		Evidence:   EvidencePacket{}, // all empty
	}
	if err := ValidateVerdict(v); err == nil {
		t.Fatal("expected error for empty evidence, got nil")
	}
}

func TestValidateVerdictInvalidVerdictType(t *testing.T) {
	v := &VerdictHandoffPacket{
		ID:         "v-3",
		DomainID:   "eval:a2a",
		Phenomenon: "test",
		Verdict:    VerdictType("bogus"),
		Evidence:   EvidencePacket{MetricRefs: []string{"x"}},
	}
	if err := ValidateVerdict(v); err == nil {
		t.Fatal("expected error for invalid verdict type, got nil")
	}
}

func TestValidateVerdictInvalidConfidence(t *testing.T) {
	v := &VerdictHandoffPacket{
		ID:         "v-4",
		DomainID:   "eval:a2a",
		Phenomenon: "test",
		Verdict:    VerdictFix,
		Evidence:   EvidencePacket{MetricRefs: []string{"x"}},
		RootCause:  RootCause{Summary: "s", Confidence: "bogus"},
	}
	if err := ValidateVerdict(v); err == nil {
		t.Fatal("expected error for invalid confidence, got nil")
	}
}
