// Package message implements selective message merge-forward (F294): bundle a
// set of messages and transfer them to a target thread, deduplicating by id and
// transferring idempotently.
package message

import (
	"errors"
	"sync"
)

// ErrEmptyTarget is returned when a forward has no target thread.
var ErrEmptyTarget = errors.New("message: empty transfer target")

// Message is a minimal transferable message.
type Message struct {
	ID   string
	Text string
}

// MessageBundle is a collection of message ids selected for forward.
type MessageBundle struct {
	IDs []string
}

// TransferTarget is where a bundle is forwarded.
type TransferTarget struct {
	ThreadID string
}

// Merger performs selective merge-forward with idempotent transfer (F294).
type Merger struct {
	mu         sync.Mutex
	transferred map[string]map[string]bool // threadID -> msgID -> done
}

// NewMerger creates a merger.
func NewMerger() *Merger { return &Merger{transferred: make(map[string]map[string]bool)} }

// Select bundles the given messages, deduplicating by ID (F294 selective).
func Select(msgs []Message) MessageBundle {
	seen := make(map[string]bool)
	var ids []string
	for _, m := range msgs {
		if !seen[m.ID] {
			seen[m.ID] = true
			ids = append(ids, m.ID)
		}
	}
	return MessageBundle{IDs: ids}
}

// Forward transfers a bundle's messages to a target thread. Returns the IDs
// actually transferred (idempotent: messages already forwarded to that target
// are skipped). Fail-closed: an empty target is rejected, and unknown message
// ids are skipped (never fabricated).
func (m *Merger) Forward(b MessageBundle, target TransferTarget, lookup func(id string) (Message, bool)) ([]string, error) {
	if target.ThreadID == "" {
		return nil, ErrEmptyTarget
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.transferred[target.ThreadID] == nil {
		m.transferred[target.ThreadID] = make(map[string]bool)
	}
	var out []string
	for _, id := range b.IDs {
		if m.transferred[target.ThreadID][id] {
			continue // idempotent
		}
		if _, ok := lookup(id); !ok {
			continue // skip unknown (fail-closed)
		}
		m.transferred[target.ThreadID][id] = true
		out = append(out, id)
	}
	return out, nil
}
