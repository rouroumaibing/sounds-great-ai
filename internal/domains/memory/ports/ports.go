package ports

// EvidenceRecord is the API-facing evidence record structure.
type EvidenceRecord struct {
	ID        string   `json:"id"`
	ThreadID  string   `json:"thread_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	CreatedAt int64    `json:"created_at"`
}

// IEvidenceStore is the port for evidence storage.
type IEvidenceStore interface {
	ListEvidence() ([]*EvidenceRecord, error)
	AddEvidence(threadID, typ, title, content string, tags []string) (*EvidenceRecord, error)
}

// IMemoryStore is the port for general memory storage.
type IMemoryStore interface {
	Store(key string, value any) error
	Load(key string) (any, bool, error)
	Delete(key string) error
}
