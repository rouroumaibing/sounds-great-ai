package sop

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sounds-great-ai/pkg/pack"
)

// findRepoRoot walks up from this source file to locate the module root (the
// directory containing go.mod), so the closure check is cwd-independent and
// runs the same way in local `go test` and in CI.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("repo root (go.mod) not found")
	return ""
}

// TestReviewClosureSeedIdentity verifies the seed pack satisfies the
// cross-model review identity model at the data level: every breed resolves to
// a non-empty executing-variant dog_id, and no two breeds resolve to the same
// dog_id. The enforced identity is the variant (model) dog_id, so this is the
// structural guarantee behind the fail-closed handoff gate — if two breeds
// collapsed to the same dog_id, the gate could not tell author from reviewer
// and the invariant would be unenforceable.
func TestReviewClosureSeedIdentity(t *testing.T) {
	root := findRepoRoot(t)
	path := filepath.Join(root, "packs", "default", "breeds", "dog-template.json")
	breeds, err := pack.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("load seed template: %v", err)
	}
	if len(breeds) == 0 {
		t.Fatal("seed template contains no breeds")
	}

	seen := make(map[string]string) // executing variant dog_id -> breed id
	for id, b := range breeds {
		v := b.DefaultVariant()
		if v == nil {
			t.Errorf("breed %q has no variant; review identity cannot be resolved", id)
			continue
		}
		if v.DogID == "" {
			t.Errorf("breed %q default variant %q has empty dog_id; cross-model review identity cannot be resolved", id, v.ID)
			continue
		}
		if other, ok := seen[v.DogID]; ok {
			t.Errorf("breed %q and %q resolve to the same executing variant dog_id %q; review handoff cannot distinguish author from reviewer", other, id, v.DogID)
		}
		seen[v.DogID] = id
	}
}

// TestReviewClosureFailClosed exercises the write-back invariant end to end:
// the assigned reviewer may record the verdict, the author (same dog_id)
// cannot, and an unassigned reviewer cannot. The gate fails closed — it blocks
// rather than allowing a review whose identity constraint is violated.
func TestReviewClosureFailClosed(t *testing.T) {
	cyc := NewReviewCycle()
	cyc.AssignReview("bianmu", "xigou", "thread-1")

	// Assigned reviewer records a verdict -> ok.
	if err := cyc.RecordReview(ReviewProvenance{
		ReviewerDogID:    "xigou",
		AuthorDogID:      "bianmu",
		ReviewerThreadID: "thread-1",
		ReviewSHA:        "sha-1",
	}); err != nil {
		t.Fatalf("assigned reviewer write-back rejected: %v", err)
	}
	if !cyc.IsCrossBreedReview("sha-1", "bianmu") {
		t.Error("expected cross-breed review to be recorded")
	}
	if cyc.IsSelfReview("sha-1", "bianmu") {
		t.Error("assigned reviewer must not be flagged as self-review")
	}

	// Author (same dog_id) writing back -> fail-closed.
	if err := cyc.RecordReview(ReviewProvenance{
		ReviewerDogID:    "bianmu",
		AuthorDogID:      "bianmu",
		ReviewerThreadID: "thread-1",
		ReviewSHA:        "sha-2",
	}); err == nil {
		t.Error("author self-review write-back must be rejected (fail-closed)")
	}

	// Unassigned reviewer writing back -> fail-closed.
	if err := cyc.RecordReview(ReviewProvenance{
		ReviewerDogID:    "demu",
		AuthorDogID:      "bianmu",
		ReviewerThreadID: "thread-1",
		ReviewSHA:        "sha-3",
	}); err == nil {
		t.Error("unassigned reviewer write-back must be rejected (fail-closed)")
	}

	// Assigned reviewer writing back to a different thread -> fail-closed.
	if err := cyc.RecordReview(ReviewProvenance{
		ReviewerDogID:    "xigou",
		AuthorDogID:      "bianmu",
		ReviewerThreadID: "thread-2",
		ReviewSHA:        "sha-4",
	}); err == nil {
		t.Error("write-back to a non-review thread must be rejected (fail-closed)")
	} else if !errors.Is(err, ErrWrongCarrier) {
		t.Errorf("expected ErrWrongCarrier, got %v", err)
	}
}

// TestReviewClosureSkillText asserts the cross-model review constraint is
// documented in the review-related skill texts, so the invariant is enforced
// not only in code but also in the operating procedure that the agents follow.
func TestReviewClosureSkillText(t *testing.T) {
	root := findRepoRoot(t)
	skills := []string{
		"request-review",
		"receive-review",
		"cross-dog-handoff",
		"merge-gate",
	}
	for _, name := range skills {
		p := filepath.Join(root, "packs", "default", "skills", name, "SKILL.md")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read skill %s: %v", name, err)
		}
		if !strings.Contains(string(data), "dog_id") {
			t.Errorf("skill %s does not document the dog_id review constraint", name)
		}
	}
	// The merge gate must document the dual-route (local / external) intent and
	// that an ambiguous intent fails closed, so the code-side guard has a
	// procedural counterpart.
	mg, err := os.ReadFile(filepath.Join(root, "packs", "default", "skills", "merge-gate", "SKILL.md"))
	if err != nil {
		t.Fatalf("read merge-gate skill: %v", err)
	}
	if !strings.Contains(string(mg), "fail-closed") {
		t.Error("merge-gate skill does not document the fail-closed dual-route intent")
	}
}
