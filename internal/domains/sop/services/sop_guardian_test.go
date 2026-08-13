package services

import (
	"testing"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/sop"
	sopPorts "sounds-great-ai/internal/domains/sop/ports"
)

func TestSOPGuardianServiceDepthExceeded(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 1))
	thread := &a2a.Thread{ID: "t", ReviewRoundCount: 1, Participants: []string{"bianmu"}}
	if got := svc.CheckA2ADepth(thread); got != sopPorts.EscalateToCVO {
		t.Fatalf("CheckA2ADepth = %v, want EscalateToCVO", got)
	}
}

func TestSOPGuardianServiceDepthOK(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	thread := &a2a.Thread{ID: "t", ReviewRoundCount: 0, Participants: []string{"bianmu"}}
	if got := svc.CheckA2ADepth(thread); got != sopPorts.Continue {
		t.Fatalf("CheckA2ADepth = %v, want Continue", got)
	}
}

func TestSOPGuardianServiceSelectReviewer(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	reviewer := svc.SelectReviewer("bianmu", []string{"xigou", "demu"}, sopPorts.ReviewPolicy{RequireDifferentBreed: true})
	if reviewer != "xigou" {
		t.Fatalf("SelectReviewer = %q, want xigou", reviewer)
	}
	// No candidate differs from author → empty.
	if got := svc.SelectReviewer("bianmu", []string{"bianmu"}, sopPorts.ReviewPolicy{RequireDifferentBreed: true}); got != "" {
		t.Fatalf("SelectReviewer = %q, want empty", got)
	}
}

func TestSOPGuardianServiceMaxDepth(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 5))
	if got := svc.MaxA2ADepth(); got != 5 {
		t.Fatalf("MaxA2ADepth = %d, want 5", got)
	}
}
