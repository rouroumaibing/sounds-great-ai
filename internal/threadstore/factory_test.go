package threadstore_test

import (
	"testing"

	"sounds-great-ai/internal/threadstore"
)

func TestNewThreadStore_InMemory(t *testing.T) {
	store, err := threadstore.NewThreadStore(threadstore.StoreConfig{})
	if err != nil {
		t.Fatalf("NewThreadStore: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
	thread, err := store.CreateThread("Test")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("thread ID is empty")
	}
}

func TestNewMessageStore_InMemory(t *testing.T) {
	store, err := threadstore.NewMessageStore(threadstore.StoreConfig{})
	if err != nil {
		t.Fatalf("NewMessageStore: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
}

func TestNewThreadStore_SQLite(t *testing.T) {
	store, err := threadstore.NewThreadStore(threadstore.StoreConfig{SQLitePath: ":memory:"})
	if err != nil {
		t.Fatalf("NewThreadStore SQLite: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
}

func TestNewMessageStore_SQLite(t *testing.T) {
	store, err := threadstore.NewMessageStore(threadstore.StoreConfig{SQLitePath: ":memory:"})
	if err != nil {
		t.Fatalf("NewMessageStore SQLite: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
}
