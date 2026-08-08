package testutil

import (
	"time"

	"sounds-great-ai/internal/threadstore"
)

// SampleThread returns a thread for testing.
func SampleThread(title string) *threadstore.Thread {
	return &threadstore.Thread{
		ID:        "thread-1",
		Title:     title,
		CreatedAt: time.Now().UnixMilli(),
	}
}

// SampleMessage returns a message for testing.
func SampleMessage(threadID, content string) *threadstore.Message {
	return &threadstore.Message{
		ID:        "msg-1",
		ThreadID:  threadID,
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// SampleMessages returns N messages for a thread.
func SampleMessages(threadID string, n int) []*threadstore.Message {
	msgs := make([]*threadstore.Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = &threadstore.Message{
			ID:        "msg-" + string(rune('a'+i)),
			ThreadID:  threadID,
			Role:      "user",
			Content:   "message " + string(rune('0'+i+1)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		}
	}
	return msgs
}
