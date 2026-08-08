package threadstore

import "encoding/json"

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

// ThreadStore is the port interface for thread + session storage.
type ThreadStore interface {
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
