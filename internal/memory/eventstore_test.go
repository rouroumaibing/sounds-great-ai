package memory

import (
	"path/filepath"
	"testing"
)

func newEventRegistry(t *testing.T) *LaneRegistry {
	t.Helper()
	dir := t.TempDir()
	reg := NewLaneRegistryAt(filepath.Join(dir, "lanes.json"))
	t.Cleanup(reg.Close)
	return reg
}

// F227 AC-A1/AC-A4: MarkEvent records a cat-declared event and teleport resolves
// the exact (thread, message) coordinate.
func TestEventMemory_MarkAndTeleport(t *testing.T) {
	reg := newEventRegistry(t)
	ev, err := reg.MarkEvent("op-1", "bianmu", "thread-a", "msg-7", "realized the coordinate system was wrong", "cognitive:reframe")
	if err != nil {
		t.Fatalf("MarkEvent: %v", err)
	}
	if ev.ID == "" {
		t.Fatal("event id should be assigned")
	}
	tp := reg.ResolveEventTeleport(ev.ID, "op-1")
	if tp == nil || tp.ThreadID != "thread-a" || tp.MessageID != "msg-7" {
		t.Fatalf("teleport should resolve exact coordinate: %+v", tp)
	}
}

// F227 fail-closed: an empty owner never stores or resolves events.
func TestEventMemory_FailClosedOnEmptyOwner(t *testing.T) {
	reg := newEventRegistry(t)
	if _, err := reg.MarkEvent("", "bianmu", "t", "m", "s", "c"); err == nil {
		t.Fatal("expected error for empty owner")
	}
	// No events recorded under empty owner → nothing resolvable.
	if tp := reg.ResolveEventTeleport("any-id", ""); tp != nil {
		t.Fatal("empty owner must not resolve")
	}
	if n := reg.EventCount(""); n != 0 {
		t.Fatalf("empty owner count must be 0, got %d", n)
	}
}

// F227 owner isolation: an event recorded by op-1 is invisible to op-2.
func TestEventMemory_OwnerIsolation(t *testing.T) {
	reg := newEventRegistry(t)
	ev, err := reg.MarkEvent("op-1", "bianmu", "t", "m", "summary", "transition")
	if err != nil {
		t.Fatal(err)
	}
	// op-2 cannot resolve op-1's event.
	if tp := reg.ResolveEventTeleport(ev.ID, "op-2"); tp != nil {
		t.Fatal("op-2 must not see op-1's event")
	}
	// op-2 timeline empty; op-1 timeline has it.
	if got := reg.EventTimeline("op-2", "", ""); len(got) != 0 {
		t.Fatalf("op-2 timeline should be empty, got %d", len(got))
	}
	if got := reg.EventTimeline("op-1", "", ""); len(got) != 1 {
		t.Fatalf("op-1 timeline should have 1, got %d", len(got))
	}
}

// F227 Phase A: magic-word event with confidence tier is recorded and filterable.
func TestEventMemory_MagicWordTimelineFilter(t *testing.T) {
	reg := newEventRegistry(t)
	// Two distinct messages both carrying the magic word (different coordinates).
	if _, err := reg.RecordMagicWordEvent("op-1", "jinmao", "t1", "m1", "脚手架", ConfHigh); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.RecordMagicWordEvent("op-1", "jinmao", "t2", "m2", "脚手架", ConfLow); err != nil {
		t.Fatal(err)
	}
	// Filter by type.
	mw := reg.EventTimeline("op-1", EventTypeMagicWord, "")
	if len(mw) != 2 {
		t.Fatalf("expected 2 magic-word events, got %d", len(mw))
	}
	// Filter by confidence (high only) drops the low one.
	high := reg.EventTimeline("op-1", EventTypeMagicWord, ConfHigh)
	if len(high) != 1 {
		t.Fatalf("expected 1 high-confidence event, got %d", len(high))
	}
	if high[0].Confidence != ConfHigh {
		t.Fatalf("expected high, got %s", high[0].Confidence)
	}
}

// F227 Phase C: resolution chain links a harness change to an event.
func TestEventMemory_ResolutionLink(t *testing.T) {
	reg := newEventRegistry(t)
	ev, err := reg.MarkEvent("op-1", "bianmu", "t", "m", "s", "c")
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.LinkEventResolution(ev.ID, "op-1", "commit:abc123"); err != nil {
		t.Fatalf("LinkEventResolution: %v", err)
	}
	evs := reg.EventTimeline("op-1", "", "")
	if len(evs) != 1 || evs[0].RelatedHarness != "commit:abc123" {
		t.Fatalf("resolution link not persisted: %+v", evs)
	}
	// Wrong owner cannot link.
	if err := reg.LinkEventResolution(ev.ID, "op-2", "commit:x"); err == nil {
		t.Fatal("wrong owner must not link resolution")
	}
}
