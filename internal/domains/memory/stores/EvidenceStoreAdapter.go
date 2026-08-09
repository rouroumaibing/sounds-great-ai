package stores

import (
	memoryPorts "sounds-great-ai/internal/domains/memory/ports"
	"sounds-great-ai/internal/memory"
)

// EvidenceStoreAdapter wraps an existing memory.EvidenceStore to implement
// the domain ports.IEvidenceStore interface.
type EvidenceStoreAdapter struct {
	inner memory.EvidenceStore
}

// NewEvidenceStoreAdapter creates a new EvidenceStoreAdapter.
func NewEvidenceStoreAdapter(inner memory.EvidenceStore) *EvidenceStoreAdapter {
	return &EvidenceStoreAdapter{inner: inner}
}

func (a *EvidenceStoreAdapter) ListEvidence() ([]*memoryPorts.EvidenceRecord, error) {
	records, err := a.inner.ListEvidence()
	if err != nil {
		return nil, err
	}
	result := make([]*memoryPorts.EvidenceRecord, len(records))
	for i, r := range records {
		result[i] = &memoryPorts.EvidenceRecord{
			ID:        r.ID,
			ThreadID:  r.ThreadID,
			Type:      r.Type,
			Title:     r.Title,
			Content:   r.Content,
			Tags:      r.Tags,
			CreatedAt: r.CreatedAt,
		}
	}
	return result, nil
}

func (a *EvidenceStoreAdapter) AddEvidence(threadID, typ, title, content string, tags []string) (*memoryPorts.EvidenceRecord, error) {
	r, err := a.inner.AddEvidence(threadID, typ, title, content, tags)
	if err != nil {
		return nil, err
	}
	return &memoryPorts.EvidenceRecord{
		ID:        r.ID,
		ThreadID:  r.ThreadID,
		Type:      r.Type,
		Title:     r.Title,
		Content:   r.Content,
		Tags:      r.Tags,
		CreatedAt: r.CreatedAt,
	}, nil
}
