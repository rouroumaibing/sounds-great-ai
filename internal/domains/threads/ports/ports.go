package ports

import (
	"encoding/json"
	"time"
)

// Thread represents a conversation thread.
type Thread struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	DeletedAt *int64 `json:"deleted_at"`
}

// SessionRecord represents a sealed/unsealed session within a thread.
type SessionRecord struct {
	ID           string `json:"id"`
	ThreadID     string `json:"thread_id"`
	BreedID      string `json:"breed_id"`
	Seq          int    `json:"seq"`
	Status       string `json:"status"`
	MessageCount int    `json:"message_count"`
	SealReason   string `json:"seal_reason"`
	CreatedAt    int64  `json:"created_at"`
	SealedAt     *int64 `json:"sealed_at"`
}

// IThreadStore is the port for thread + session storage.
type IThreadStore interface {
	CreateThread(title string) (*Thread, error)
	ListThreads() ([]*Thread, error)
	DeleteThread(id string) error
	UpdateTitle(id string, title string) error
	AddEvent(threadID string, event json.RawMessage) error
	GetEvents(threadID string) ([]json.RawMessage, error)
	CreateSession(threadID, breedID string) (*SessionRecord, error)
	ListSessions(threadID string) ([]*SessionRecord, error)
	UnsealSession(sessionID string) error
}

// IMessageStore is the port for message storage. Mirrors the flat
// threadstore.MessageStore interface (Append / GetByThread / GetByThreadBefore)
// so the adapter wraps the existing store without behavior change.
type IMessageStore interface {
	Append(msg *Message) error
	GetByThread(threadID string, limit int) ([]*Message, error)
	GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*Message, error)
}

// Message represents a chat message. Mirrors the flat threadstore.Message
// shape so adapters can map 1:1 and preserve the exact JSON contract.
type Message struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"thread_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Sender    string    `json:"sender"`
	Timestamp time.Time `json:"timestamp"`
}
