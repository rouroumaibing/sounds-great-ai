package threadstore_test

import (
	"encoding/json"
	"testing"

	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/testutil"
)

func TestInMemoryThreadStoreContract(t *testing.T) {
	testutil.RunThreadStoreContract(t, threadstore.NewInMemoryThreadStore())
}

func TestInMemoryThreadStore_AddAndGetEvents(t *testing.T) {
	s := threadstore.NewInMemoryThreadStore()
	thread, _ := s.CreateThread("Event Test")
	event := json.RawMessage(`{"type":"test"}`)
	if err := s.AddEvent(thread.ID, event); err != nil {
		t.Fatalf("AddEvent: %v", err)
	}
	events, err := s.GetEvents(thread.ID)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
}

func TestInMemoryThreadStore_UpdateTitleNotFound(t *testing.T) {
	s := threadstore.NewInMemoryThreadStore()
	err := s.UpdateTitle("nonexistent", "title")
	if err == nil {
		t.Fatal("expected error for nonexistent thread")
	}
}

func TestInMemoryThreadStore_ListSessionsEmpty(t *testing.T) {
	s := threadstore.NewInMemoryThreadStore()
	thread, _ := s.CreateThread("Empty Sessions")
	sessions, err := s.ListSessions(thread.ID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("got %d sessions, want 0", len(sessions))
	}
}

func TestInMemoryThreadStore_UnsealSessionNotFound(t *testing.T) {
	s := threadstore.NewInMemoryThreadStore()
	err := s.UnsealSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}
