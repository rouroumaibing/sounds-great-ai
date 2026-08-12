package threadstore

import "time"

// Message represents a single chat message in a conversation thread.
// Simplified message record (no idempotency, no delivery status).
type Message struct {
	ID        string    // unique message ID (auto-generated)
	ThreadID  string    // conversation thread key (uses sessionID)
	Role      string    // "user" | "assistant"
	Content   string    // message text
	Sender    string    // breed ID for assistant messages, "" for user
	Timestamp time.Time // when the message was created
}

// MessageStore is the port interface for chat message persistence.
// Simplified port interface (40+ methods reduced to 3).
type MessageStore interface {
	Append(msg *Message) error
	GetByThread(threadID string, limit int) ([]*Message, error)
	// GetByThreadBefore returns up to limit messages older than (before, beforeID),
	// in ascending order (oldest first). If before is zero time, returns the most
	// recent messages. Same-timestamp messages use ID lexicographic ordering as tiebreaker.
	GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*Message, error)
}
