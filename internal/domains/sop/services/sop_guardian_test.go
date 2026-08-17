package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sounds-great-ai/internal/a2a"
	"sounds-great-ai/internal/sop"
	sopPorts "sounds-great-ai/internal/domains/sop/ports"
)

// writeTempDefinition writes a minimal SOP definition with a `review` stage
// blocker (reviewer_not_author) and returns the path.
func writeTempDefinition(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "development.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write temp definition: %v", err)
	}
	return path
}

const reviewStageYAML = `id: development
domain: engineering
label: Development SOP
stages:
  - id: review
    label: 独立验证
    hard_rules:
      - id: review-no-self-review
        text: 同一个体不能 review 自己的代码
        severity: blocker
        predicate:
          type: handle_check
          constraint: reviewer_not_author
`

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

func TestEnforceReviewHandoffSelfReviewBlocked(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	// Baseline invariant: a dog cannot review its own work, even without a
	// definition file present.
	got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
	})
	if !got.Blocked {
		t.Fatal("expected self-review to be blocked")
	}
	if len(got.Messages) == 0 {
		t.Error("expected a blocking message")
	}
}

func TestEnforceReviewHandoffSameDogIDBlocked(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	// Identity, not breed, governs the invariant: two different breeds that
	// resolve to the same dog_id cannot review each other. A handoff that
	// collapses author and reviewer to one identity is blocked fail-closed.
	got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu-sonnet",
		ReviewerBreed: "xigou", ReviewerDogID: "bianmu-sonnet",
	})
	if !got.Blocked {
		t.Fatal("expected same-dog_id handoff to be blocked regardless of breed")
	}
}

func TestEnforceReviewHandoffCrossBreedAllowed(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-dog",
	})
	if got.Blocked {
		t.Fatalf("expected cross-breed handoff allowed, got blocked: %v", got.Messages)
	}
}

func TestEnforceReviewHandoffYAMLBlocker(t *testing.T) {
	path := writeTempDefinition(t, reviewStageYAML)
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	svc.SetDefinitionPath(path)

	// reviewer_not_author blocker: same dog → blocked.
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
	}); !got.Blocked {
		t.Error("expected YAML blocker to reject self-review")
	}
	// different dog → allowed.
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-dog",
	}); got.Blocked {
		t.Errorf("expected cross-breed allowed, got blocked: %v", got.Messages)
	}
}

func TestEnforceReviewHandoffWriteBackAssigned(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	session := "sess-1"
	// First cross-dog handoff = review request (assign bianmu -> xigou-dog).
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-dog",
		SessionID:    session,
	}); got.Blocked {
		t.Fatalf("review request should be allowed, got %v", got.Messages)
	}
	// Return trip by the assigned reviewer: recorded and allowed.
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "xigou", AuthorDogID: "xigou-dog",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
		SessionID:    session,
	}); got.Blocked {
		t.Fatalf("assigned reviewer write-back should be allowed, got %v", got.Messages)
	}
	// A different dog attempting the write-back is blocked (wrong principal).
	svc2 := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	svc2.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-dog",
		SessionID:    session,
	})
	if got := svc2.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "demu", AuthorDogID: "demu-dog",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
		SessionID:    session,
	}); !got.Blocked {
		t.Fatal("non-assigned reviewer write-back should be blocked")
	}
}

func TestEnforceReviewHandoffTerminalRouteReasons(t *testing.T) {
	session := "sess-L"
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	// Issue the lease: bianmu -> xigou-dog in sess-L.
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-dog",
		SessionID: session,
	}); got.Blocked {
		t.Fatalf("review request should be allowed, got %v", got.Messages)
	}

	// Assigned reviewer returns the verdict in a DIFFERENT thread -> carrier
	// mismatch (holder/target thread mismatch).
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "xigou", AuthorDogID: "xigou-dog",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
		SessionID:    "other-thread",
	}); !got.Blocked {
		t.Fatal("cross-thread write-back must be blocked")
	} else if !strings.Contains(strings.Join(got.Messages, " "), "thread_mismatch") {
		t.Errorf("expected a thread_mismatch reason, got %v", got.Messages)
	}

	// Stale generation -> generation_mismatch.
	if got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "xigou", AuthorDogID: "xigou-dog",
		ReviewerBreed: "bianmu", ReviewerDogID: "bianmu",
		SessionID:    session, Generation: 999,
	}); !got.Blocked {
		t.Fatal("stale-generation write-back must be blocked")
	} else if !strings.Contains(strings.Join(got.Messages, " "), "generation_mismatch") {
		t.Errorf("expected generation_mismatch, got %v", got.Messages)
	}
}

func TestEnforceReviewHandoffModelIdentityViaVariant(t *testing.T) {
	svc := NewSOPGuardianService(sop.NewGuardian(nil, 3))
	// The handoff carries the executing variant (model) dog_id. Two breeds
	// with distinct model identities are allowed to review each other even if
	// their breed labels would otherwise suggest a relationship.
	got := svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu-sonnet",
		ReviewerBreed: "xigou", ReviewerDogID: "xigou-opus",
	})
	if got.Blocked {
		t.Fatalf("distinct model identities should be allowed, got %v", got.Messages)
	}
	// The same model identity across breeds collapses to one reviewer -> blocked.
	got = svc.EnforceReviewHandoff(sopPorts.ReviewHandoffInput{
		AuthorBreed: "bianmu", AuthorDogID: "bianmu-sonnet",
		ReviewerBreed: "xigou", ReviewerDogID: "bianmu-sonnet",
	})
	if !got.Blocked {
		t.Fatal("same model identity must block regardless of breed")
	}
}
