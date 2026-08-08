package memory_test

import (
	"testing"

	"sounds-great-ai/internal/memory"
	"sounds-great-ai/testutil"
)

func TestInMemoryEvidenceStoreContract(t *testing.T) {
	testutil.RunEvidenceStoreContract(t, memory.NewEvidenceStore())
}

func TestInMemoryEvidenceStore_MultipleAdd(t *testing.T) {
	store := memory.NewEvidenceStore()
	for i := 0; i < 5; i++ {
		_, err := store.AddEvidence("thread-1", "bug", "title", "content", nil)
		if err != nil {
			t.Fatalf("AddEvidence %d: %v", i, err)
		}
	}
	records, err := store.ListEvidence()
	if err != nil {
		t.Fatalf("ListEvidence: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("got %d records, want 5", len(records))
	}
}

func TestInMemoryEvidenceStore_EmptyTags(t *testing.T) {
	store := memory.NewEvidenceStore()
	rec, err := store.AddEvidence("t1", "note", "title", "content", nil)
	if err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}
	if rec.Tags != nil && len(rec.Tags) != 0 {
		t.Errorf("Tags = %v, want nil or empty", rec.Tags)
	}
}
