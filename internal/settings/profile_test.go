package settings

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileRepository_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	repo := NewProfileRepository(dir, "operator")

	key := "family-sounds-great-ai"
	c := &RelationshipCapsule{
		RelationshipKey: key,
		OwnerDog:        "bianmu",
		Status:          "active",
		SourceRef:       "operator:manual",
		CorrectionPath:  "was over-eager; operator said pace down",
		Body:            "Operator prefers terse replies. Likes diagrams. Hates filler.",
	}
	if err := repo.WriteCapsule(c); err != nil {
		t.Fatalf("WriteCapsule: %v", err)
	}

	got, ok, err := repo.ReadCapsule(key)
	if err != nil {
		t.Fatalf("ReadCapsule: %v", err)
	}
	if !ok {
		t.Fatal("expected capsule to exist")
	}
	if got.Body != c.Body {
		t.Errorf("body = %q, want %q", got.Body, c.Body)
	}
	if got.OwnerDog != "bianmu" || got.Status != "active" || got.SourceRef != "operator:manual" {
		t.Errorf("front-matter mismatch: %+v", got)
	}
	if got.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set on write")
	}

	keys, err := repo.ListCapsules()
	if err != nil {
		t.Fatalf("ListCapsules: %v", err)
	}
	if len(keys) != 1 || keys[0] != key {
		t.Errorf("ListCapsules = %v, want [%s]", keys, key)
	}

	if err := repo.DeleteCapsule(key); err != nil {
		t.Fatalf("DeleteCapsule: %v", err)
	}
	if _, ok, _ := repo.ReadCapsule(key); ok {
		t.Error("capsule should be gone after delete")
	}
}

func TestProfileRepository_UnsafeKeySanitized(t *testing.T) {
	dir := t.TempDir()
	repo := NewProfileRepository(dir, "operator")
	// A key with path separators / traversal must not escape the relationship dir.
	key := "../../etc/passwd"
	if err := repo.WriteCapsule(&RelationshipCapsule{RelationshipKey: key, Body: "x"}); err != nil {
		t.Fatalf("WriteCapsule: %v", err)
	}
	// The file must live INSIDE the relationship directory, not at /etc/passwd.
	entries, _ := filepath.Glob(filepath.Join(repo.root, "*"+capsuleSuffix))
	if len(entries) != 1 {
		t.Fatalf("expected exactly one capsule file under %s, got %v", repo.root, entries)
	}
}

func TestProfileRepository_NoFrontMatterFallback(t *testing.T) {
	dir := t.TempDir()
	repo := NewProfileRepository(dir, "operator")
	key := "legacy-capsule"
	// Hand-write a primer without front-matter (lenient import path).
	if err := writeAtomicRaw(repo.capsulePath(key), []byte("operator likes short answers"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, ok, err := repo.ReadCapsule(key)
	if err != nil || !ok {
		t.Fatalf("ReadCapsule ok=%v err=%v", ok, err)
	}
	if got.Body != "operator likes short answers" {
		t.Errorf("body = %q", got.Body)
	}
	if got.Status != "active" {
		t.Errorf("default status = %q, want active", got.Status)
	}
}

func TestProfileRepository_BodyCapEnforced(t *testing.T) {
	repo := NewProfileRepository(t.TempDir(), "operator")
	// 301 runes must be rejected; exactly 300 is allowed.
	tooLong := strings.Repeat("x", MaxCapsuleBodyLen+1)
	if err := repo.WriteCapsule(&RelationshipCapsule{RelationshipKey: "k", Body: tooLong}); !errors.Is(err, ErrCapsuleTooLong) {
		t.Fatalf("expected ErrCapsuleTooLong, got %v", err)
	}
	exact := strings.Repeat("y", MaxCapsuleBodyLen)
	if err := repo.WriteCapsule(&RelationshipCapsule{RelationshipKey: "k", Body: exact}); err != nil {
		t.Fatalf("300-rune body should be allowed: %v", err)
	}
}

func TestProfileRepository_ApprovalHub(t *testing.T) {
	repo := NewProfileRepository(t.TempDir(), "operator")
	key := "family-sg"

	// Seed an active capsule.
	if err := repo.WriteCapsule(&RelationshipCapsule{RelationshipKey: key, Body: "v1", SourceRef: "operator:manual"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if pending, _ := repo.HasProposal(key); pending {
		t.Fatal("should have no proposal initially")
	}

	// Propose a candidate.
	if err := repo.WriteProposal(key, &RelationshipCapsule{RelationshipKey: key, Body: "v2 proposed", SourceRef: "dog:bianmu#t1"}); err != nil {
		t.Fatalf("WriteProposal: %v", err)
	}
	if pending, _ := repo.HasProposal(key); !pending {
		t.Fatal("proposal should be pending")
	}
	prop, ok, _ := repo.ReadProposal(key)
	if !ok || prop.Body != "v2 proposed" {
		t.Fatalf("proposal body mismatch: ok=%v %+v", ok, prop)
	}

	// Approve promotes the proposal to active and clears it.
	active, err := repo.ApproveProposal(key)
	if err != nil {
		t.Fatalf("ApproveProposal: %v", err)
	}
	if active.Body != "v2 proposed" {
		t.Errorf("active body = %q, want proposed text", active.Body)
	}
	if active.EvalApprovals != 1 {
		t.Errorf("EvalApprovals = %d, want 1", active.EvalApprovals)
	}
	if pending, _ := repo.HasProposal(key); pending {
		t.Error("proposal should be cleared after approval")
	}
	// Active capsule should still be readable with the promoted body.
	got, _, _ := repo.ReadCapsule(key)
	if got.Body != "v2 proposed" || got.SourceRef != "dog:bianmu#t1" {
		t.Errorf("promoted capsule mismatch: %+v", got)
	}

	// Reject path: new proposal then reject bumps the rejection counter.
	if err := repo.WriteProposal(key, &RelationshipCapsule{RelationshipKey: key, Body: "v3 bad"}); err != nil {
		t.Fatalf("WriteProposal#2: %v", err)
	}
	if _, err := repo.RejectProposal(key); err != nil {
		t.Fatalf("RejectProposal: %v", err)
	}
	got, _, _ = repo.ReadCapsule(key)
	if got.EvalRejections != 1 {
		t.Errorf("EvalRejections = %d, want 1", got.EvalRejections)
	}
	if pending, _ := repo.HasProposal(key); pending {
		t.Error("proposal should be cleared after rejection")
	}

	// Approving with no pending proposal is a conflict-style error.
	if _, err := repo.ApproveProposal(key); err == nil {
		t.Error("expected error approving with no pending proposal")
	}
}
