package settings

import (
	"testing"
)

func TestContinuityStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewContinuityStore(dir)

	breed := "bianmu"
	if err := store.Record(breed, "正在重构 settings 层", "thread-42"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	summary, ok, err := store.LastDigest(breed)
	if err != nil {
		t.Fatalf("LastDigest: %v", err)
	}
	if !ok {
		t.Fatal("expected digest to exist")
	}
	if summary != "正在重构 settings 层" {
		t.Errorf("summary = %q", summary)
	}

	// Overwrite updates in place.
	if err := store.Record(breed, "继续推进 CLI adapter", "thread-43"); err != nil {
		t.Fatalf("Record#2: %v", err)
	}
	summary, _, _ = store.LastDigest(breed)
	if summary != "继续推进 CLI adapter" {
		t.Errorf("after overwrite summary = %q", summary)
	}

	keys, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0] != breed {
		t.Errorf("List = %v, want [%s]", keys, breed)
	}

	if err := store.Clear(breed); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok, _ := store.LastDigest(breed); ok {
		t.Error("digest should be gone after clear")
	}
}

func TestContinuityStore_EmptyRejected(t *testing.T) {
	store := NewContinuityStore(t.TempDir())
	if err := store.Record("", "x", ""); err == nil {
		t.Error("expected error on empty breedID")
	}
	if err := store.Record("dog", "", ""); err == nil {
		t.Error("expected error on empty summary")
	}
}

func TestContinuityStore_RotationCheckpoints(t *testing.T) {
	store := NewContinuityStore(t.TempDir())
	breed := "xigou"

	// One-shot degenerate case: rotation 0 only.
	if err := store.RecordRotation(breed, "rotation-0 note", "t0", 0); err != nil {
		t.Fatalf("RecordRotation#0: %v", err)
	}
	// A long (warm) session would record further rotations.
	if err := store.RecordRotation(breed, "rotation-1 note", "t1", 1); err != nil {
		t.Fatalf("RecordRotation#1: %v", err)
	}

	// LastDigest returns the latest rotation's summary.
	if s, ok, _ := store.LastDigest(breed); !ok || s != "rotation-1 note" {
		t.Fatalf("LastDigest = %q ok=%v, want rotation-1 note", s, ok)
	}
	// Per-rotation lookup.
	if s, ok, _ := store.LastDigestForRotation(breed, 0); !ok || s != "rotation-0 note" {
		t.Fatalf("LastDigestForRotation(0) = %q ok=%v", s, ok)
	}
	if s, ok, _ := store.LastDigestForRotation(breed, 1); !ok || s != "rotation-1 note" {
		t.Fatalf("LastDigestForRotation(1) = %q ok=%v", s, ok)
	}
	// A rotation that was never recorded is not an error, just absent.
	if _, ok, _ := store.LastDigestForRotation(breed, 9); ok {
		t.Error("rotation 9 should be absent")
	}

	// GetDoc exposes the full ring.
	doc, ok, err := store.GetDoc(breed)
	if err != nil || !ok {
		t.Fatalf("GetDoc ok=%v err=%v", ok, err)
	}
	if len(doc.Checkpoints) != 2 {
		t.Errorf("checkpoints = %d, want 2", len(doc.Checkpoints))
	}

	// Overwriting a rotation in place replaces only that checkpoint.
	if err := store.RecordRotation(breed, "rotation-0 revised", "t0b", 0); err != nil {
		t.Fatalf("RecordRotation#0b: %v", err)
	}
	if s, ok, _ := store.LastDigestForRotation(breed, 0); !ok || s != "rotation-0 revised" {
		t.Fatalf("after overwrite LastDigestForRotation(0) = %q ok=%v", s, ok)
	}
	if _, ok, _ = store.LastDigestForRotation(breed, 1); !ok {
		t.Error("rotation 1 should survive the rotation-0 overwrite")
	}
}

func TestContinuityStore_LegacyFormatMigration(t *testing.T) {
	store := NewContinuityStore(t.TempDir())
	breed := "legacy-breed"
	// Write the pre-rotation single-digest envelope directly.
	legacy := ContinuityDigest{BreedID: breed, Summary: "old single note", ThreadID: "tX", UpdatedAt: 123}
	if err := writeAtomic(store.digestPath(breed), legacy, 0o644); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	doc, ok, err := store.GetDoc(breed)
	if err != nil || !ok {
		t.Fatalf("GetDoc ok=%v err=%v", ok, err)
	}
	if len(doc.Checkpoints) != 1 || doc.Checkpoints[0].RotationIndex != 0 {
		t.Fatalf("legacy should migrate to a single rotation-0 checkpoint: %+v", doc.Checkpoints)
	}
	if s, _, _ := store.LastDigest(breed); s != "old single note" {
		t.Errorf("LastDigest = %q, want legacy summary", s)
	}
}
