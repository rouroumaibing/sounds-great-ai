package memory

import "testing"

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
