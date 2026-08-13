package stores

import (
	"time"

	threadPorts "sounds-great-ai/internal/domains/threads/ports"
	"sounds-great-ai/internal/threadstore"
)

// MessageStoreAdapter wraps an existing threadstore.MessageStore to implement
// the domain ports.IMessageStore interface. It maps 1:1 to the flat store so the
// JSON contract (id/thread_id/role/content/sender/timestamp) is preserved.
type MessageStoreAdapter struct {
	inner threadstore.MessageStore
}

// NewMessageStoreAdapter creates a new MessageStoreAdapter.
func NewMessageStoreAdapter(inner threadstore.MessageStore) *MessageStoreAdapter {
	return &MessageStoreAdapter{inner: inner}
}

func (a *MessageStoreAdapter) Append(msg *threadPorts.Message) error {
	return a.inner.Append(&threadstore.Message{
		ID:        msg.ID,
		ThreadID:  msg.ThreadID,
		Role:      msg.Role,
		Content:   msg.Content,
		Sender:    msg.Sender,
		Timestamp: msg.Timestamp,
	})
}

func (a *MessageStoreAdapter) GetByThread(threadID string, limit int) ([]*threadPorts.Message, error) {
	msgs, err := a.inner.GetByThread(threadID, limit)
	if err != nil {
		return nil, err
	}
	return mapMessages(msgs), nil
}

func (a *MessageStoreAdapter) GetByThreadBefore(threadID string, before time.Time, beforeID string, limit int) ([]*threadPorts.Message, error) {
	msgs, err := a.inner.GetByThreadBefore(threadID, before, beforeID, limit)
	if err != nil {
		return nil, err
	}
	return mapMessages(msgs), nil
}

func mapMessages(in []*threadstore.Message) []*threadPorts.Message {
	out := make([]*threadPorts.Message, len(in))
	for i, m := range in {
		out[i] = &threadPorts.Message{
			ID:        m.ID,
			ThreadID:  m.ThreadID,
			Role:      m.Role,
			Content:   m.Content,
			Sender:    m.Sender,
			Timestamp: m.Timestamp,
		}
	}
	return out
}
