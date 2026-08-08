package hooks

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestTraceStore_PersistAndGet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trace.db")
	store, err := NewTraceStore(dbPath)
	if err != nil {
		t.Fatalf("NewTraceStore: %v", err)
	}
	defer store.Close()

	events := []TraceEvent{
		{HookID: "S1", Status: "fired", ContentHash: "abc123", TokenEstimate: 50, Timestamp: time.Now()},
		{HookID: "S2", Status: "skipped", ReasonCode: "no_data", Timestamp: time.Now()},
	}

	err = store.Persist("thread-1", "turn-1", events)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	got, err := store.GetSummary("thread-1", "turn-1")
	if err != nil {
		t.Fatalf("GetSummary: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("events count = %d, want 2", len(got))
	}
	if got[0].HookID != "S1" {
		t.Errorf("first event HookID = %q, want S1", got[0].HookID)
	}
}

func TestTraceStore_ListTurns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "trace.db")
	store, _ := NewTraceStore(dbPath)
	defer store.Close()

	for i := 0; i < 3; i++ {
		store.Persist("thread-1", fmt.Sprintf("turn-%d", i), []TraceEvent{
			{HookID: "S1", Status: "fired", Timestamp: time.Now()},
		})
	}

	turns, err := store.ListTurns("thread-1", 10)
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(turns) != 3 {
		t.Errorf("turns count = %d, want 3", len(turns))
	}
}
