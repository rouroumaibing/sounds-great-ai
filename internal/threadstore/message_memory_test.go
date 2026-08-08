package threadstore_test

import (
	"testing"
	"time"

	"sounds-great-ai/internal/threadstore"
	"sounds-great-ai/testutil"
)

func TestMemoryMessageStoreContract(t *testing.T) {
	testutil.RunMessageStoreContract(t, threadstore.NewMemoryMessageStore())
}

func TestMemoryMessageStore_Limit(t *testing.T) {
	s := threadstore.NewMemoryMessageStore()
	for i := 0; i < 10; i++ {
		_ = s.Append(&threadstore.Message{
			ID:        "msg-" + string(rune('a'+i)),
			ThreadID:  "t1",
			Role:      "user",
			Content:   "msg",
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}
	msgs, err := s.GetByThread("t1", 3)
	if err != nil {
		t.Fatalf("GetByThread: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}
}

func TestMemoryMessageStore_AutoID(t *testing.T) {
	s := threadstore.NewMemoryMessageStore()
	msg := &threadstore.Message{ThreadID: "t1", Role: "user", Content: "hello"}
	if err := s.Append(msg); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if msg.ID == "" {
		t.Fatal("auto-generated ID is empty")
	}
}

func TestMemoryMessageStore_AutoTimestamp(t *testing.T) {
	s := threadstore.NewMemoryMessageStore()
	msg := &threadstore.Message{ID: "m1", ThreadID: "t1", Role: "user", Content: "hello"}
	if err := s.Append(msg); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if msg.Timestamp.IsZero() {
		t.Fatal("auto-generated timestamp is zero")
	}
}
