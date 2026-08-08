package memory

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

// EvidenceStore is the port interface for evidence storage.
type EvidenceStore interface {
	ListEvidence() ([]*EvidenceRecord, error)
	AddEvidence(threadID, typ, title, content string, tags []string) (*EvidenceRecord, error)
}
