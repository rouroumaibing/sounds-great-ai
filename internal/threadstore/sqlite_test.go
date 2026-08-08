package threadstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteMessageStoreAppendAndGet(t *testing.T) {
	t.Parallel()
	path := tempDBPath(t)
	store, err := NewSQLiteMessageStore(path)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.(*sqliteMessageStore).Close()

	msg := &Message{
		ThreadID:  "thread-1",
		Role:      "user",
		Content:   "hello",
		Timestamp: time.Now(),
	}

	if err := store.Append(msg); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if msg.ID == "" {
		t.Error("expected auto-generated ID")
	}

	msgs, err := store.GetByThread("thread-1", 0)
	if err != nil {
		t.Fatalf("GetByThread failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", msgs[0].Content)
	}
}

func TestSQLiteMessageStoreThreadIsolation(t *testing.T) {
	t.Parallel()
	path := tempDBPath(t)
	store, _ := NewSQLiteMessageStore(path)
	defer store.(*sqliteMessageStore).Close()

	store.Append(&Message{ThreadID: "a", Role: "user", Content: "a1"})
	store.Append(&Message{ThreadID: "b", Role: "user", Content: "b1"})
	store.Append(&Message{ThreadID: "a", Role: "assistant", Content: "a2"})

	msgsA, _ := store.GetByThread("a", 0)
	msgsB, _ := store.GetByThread("b", 0)

	if len(msgsA) != 2 {
		t.Errorf("expected 2 in thread-a, got %d", len(msgsA))
	}
	if len(msgsB) != 1 {
		t.Errorf("expected 1 in thread-b, got %d", len(msgsB))
	}
}

func TestSQLiteMessageStoreLimit(t *testing.T) {
	t.Parallel()
	path := tempDBPath(t)
	store, _ := NewSQLiteMessageStore(path)
	defer store.(*sqliteMessageStore).Close()

	for i := 0; i < 10; i++ {
		store.Append(&Message{
			ThreadID:  "t1",
			Role:      "user",
			Content:   "msg",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	msgs, _ := store.GetByThread("t1", 3)
	if len(msgs) != 3 {
		t.Fatalf("expected 3, got %d", len(msgs))
	}
}

func TestSQLiteMessageStorePersistence(t *testing.T) {
	path := tempDBPath(t)

	// Write
	store1, _ := NewSQLiteMessageStore(path)
	store1.Append(&Message{ThreadID: "t1", Role: "user", Content: "persistent"})
	store1.(*sqliteMessageStore).Close()

	// Reopen and read
	store2, _ := NewSQLiteMessageStore(path)
	defer store2.(*sqliteMessageStore).Close()

	msgs, _ := store2.GetByThread("t1", 0)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after reopen, got %d", len(msgs))
	}
	if msgs[0].Content != "persistent" {
		t.Errorf("expected 'persistent', got %q", msgs[0].Content)
	}
}

func TestFactoryMessageStoreInMemory(t *testing.T) {
	store, err := NewMessageStore(StoreConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*memoryMessageStore); !ok {
		t.Error("expected memoryMessageStore")
	}
}

func TestFactoryMessageStoreSQLite(t *testing.T) {
	path := tempDBPath(t)
	store, err := NewMessageStore(StoreConfig{SQLitePath: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*sqliteMessageStore); !ok {
		t.Error("expected sqliteMessageStore")
	}
	store.(*sqliteMessageStore).Close()
}

func TestFactoryThreadStoreInMemory(t *testing.T) {
	store, err := NewThreadStore(StoreConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*InMemoryThreadStore); !ok {
		t.Error("expected InMemoryThreadStore")
	}
}

func TestFactoryThreadStoreSQLite(t *testing.T) {
	path := tempDBPath(t)
	store, err := NewThreadStore(StoreConfig{SQLitePath: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := store.(*sqliteThreadStore); !ok {
		t.Error("expected sqliteThreadStore")
	}
	store.(*sqliteThreadStore).Close()
}

func tempDBPath(t *testing.T) string {
	dir := t.TempDir()
	return filepath.Join(dir, "test.db")
}

// Ensure temp dir is cleaned up (Go testing does this automatically)
var _ = os.TempDir
