package memory

import (
	"path/filepath"
	"testing"
)

func TestEvidenceStore(t *testing.T) {
	store := NewMemoryStore()
	e := Evidence{ID: "ev1", Breed: "xigou", Content: "Found nil pointer in line 42", Task: "bug-fix-123"}
	store.AddEvidence(e)
	results := store.QueryEvidence("nil pointer")
	if len(results) != 1 { t.Fatalf("expected 1 evidence, got %d", len(results)) }
	if results[0].Breed != "xigou" { t.Errorf("breed = %s, want xigou", results[0].Breed) }
}

func TestDecisionStore(t *testing.T) {
	store := NewMemoryStore()
	d := Decision{ID: "dec1", Topic: "use-reference-architecture", Decision: "Full alignment with reference design", Reason: "Hard Rails, Soft Power is better"}
	store.AddDecision(d)
	got := store.GetDecision("dec1")
	if got == nil { t.Fatal("expected decision dec1") }
	if got.Topic != "use-reference-architecture" { t.Errorf("topic = %s", got.Topic) }
}

// TestMemoryStore_PersistRoundTrip verifies the Persistent Identity layer:
// evidence/decisions written to a file-backed store survive a reopen.
func TestMemoryStore_PersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	s1 := NewMemoryStoreAt(path)
	s1.AddEvidence(Evidence{ID: "ev1", Breed: "xigou", Content: "nil deref in handler"})
	s1.AddLesson(Lesson{ID: "l1", Content: "always wrap CLI spawn in supervisor", Context: "ops"})
	s1.AddDecision(Decision{ID: "dec1", Topic: "use-reference-architecture", Decision: "align", Reason: "rails"})

	s2 := NewMemoryStoreAt(path)
	if got := s2.QueryEvidence("nil deref"); len(got) != 1 {
		t.Fatalf("evidence not persisted: want 1, got %d", len(got))
	}
	if got := s2.GetDecision("dec1"); got == nil || got.Topic != "use-reference-architecture" {
		t.Fatalf("decision not persisted")
	}
	if len(s2.lessons) != 1 {
		t.Fatalf("lesson not persisted: want 1, got %d", len(s2.lessons))
	}
}

// TestEvidenceStore_PersistRoundTrip verifies the experience store used by
// /api/memory/evidence survives restarts when file-backed.
func TestEvidenceStore_PersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.json")

	s1 := NewEvidenceStoreAt(path)
	rec, err := s1.AddEvidence("th1", "bug", "crash on spawn", "content", []string{"cli"})
	if err != nil {
		t.Fatal(err)
	}

	s2 := NewEvidenceStoreAt(path)
	list, err := s2.ListEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != rec.ID {
		t.Fatalf("evidence record not persisted: got %d records", len(list))
	}
}
