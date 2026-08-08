package threadstore

import (
	"testing"
	"time"
)

func TestMemoryMessageStoreAppendAndGet(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()

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

func TestMemoryMessageStoreThreadIsolation(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()

	store.Append(&Message{ThreadID: "thread-a", Role: "user", Content: "a1"})
	store.Append(&Message{ThreadID: "thread-b", Role: "user", Content: "b1"})
	store.Append(&Message{ThreadID: "thread-a", Role: "assistant", Content: "a2"})

	msgsA, _ := store.GetByThread("thread-a", 0)
	msgsB, _ := store.GetByThread("thread-b", 0)

	if len(msgsA) != 2 {
		t.Errorf("expected 2 messages in thread-a, got %d", len(msgsA))
	}
	if len(msgsB) != 1 {
		t.Errorf("expected 1 message in thread-b, got %d", len(msgsB))
	}
}

func TestMemoryMessageStoreLimit(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()

	for i := 0; i < 10; i++ {
		store.Append(&Message{
			ThreadID:  "thread-1",
			Role:      "user",
			Content:   "msg",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	msgs, _ := store.GetByThread("thread-1", 3)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	// Should return most recent 3
	if msgs[0].ID == "msg-1" {
		t.Error("expected most recent messages, not oldest")
	}
}

func TestMemoryMessageStoreEmptyThread(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()
	msgs, err := store.GetByThread("nonexistent", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgs != nil {
		t.Error("expected nil for empty thread")
	}
}

func TestMemoryMessageStoreEmptyThreadID(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()
	err := store.Append(&Message{Role: "user", Content: "test"})
	if err == nil {
		t.Error("expected error for empty threadID")
	}
}

func TestMemoryMessageStoreOrdering(t *testing.T) {
	t.Parallel()
	store := NewMemoryMessageStore()

	for i := 0; i < 5; i++ {
		store.Append(&Message{
			ThreadID:  "thread-1",
			Role:      "user",
			Content:   string(rune('a' + i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	msgs, _ := store.GetByThread("thread-1", 0)
	// Should be in chronological order (oldest → newest)
	for i := 1; i < len(msgs); i++ {
		if !msgs[i].Timestamp.After(msgs[i-1].Timestamp) && !msgs[i].Timestamp.Equal(msgs[i-1].Timestamp) {
			t.Errorf("messages not in chronological order at index %d", i)
		}
	}
}
