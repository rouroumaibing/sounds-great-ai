package memory

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCueMemoryRanksByRelevance(t *testing.T) {
	dir := t.TempDir()
	reg := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	defer reg.Close()

	// Old decision (unrelated to hint).
	old := reg.Lane(LaneDecision).Submit("we decided to use Postgres for the main store", "s:1")
	old.Timestamp = time.Now().Add(-60 * 24 * time.Hour).UnixMilli()
	reg.Lane(LaneDecision).Approve(old.ID)

	// New lesson tightly related to the hint "database indexing".
	fresh := reg.Lane(LaneLesson).Submit("lesson: add database indexing to avoid slow queries", "s:2")
	fresh.Timestamp = time.Now().UnixMilli()
	reg.Lane(LaneLesson).Approve(fresh.ID)

	block, ok, err := reg.CueMemory(20, "", "database indexing strategy")
	if err != nil || !ok {
		t.Fatalf("CueMemory: %v %v", ok, err)
	}
	// The relevant lesson must outrank the old unrelated decision.
	idxFresh := strings.Index(block, "database indexing")
	idxOld := strings.Index(block, "Postgres")
	if idxFresh == -1 || idxOld == -1 {
		t.Fatalf("both entries should appear: %q", block)
	}
	if idxFresh > idxOld {
		t.Fatalf("relevant lesson should rank before old decision: %q", block)
	}
}

func TestCueMemoryEmpty(t *testing.T) {
	reg := NewLaneRegistry()
	if _, ok, _ := reg.CueMemory(20, "", "x"); ok {
		t.Fatalf("expected no cue block for empty registry")
	}
}
