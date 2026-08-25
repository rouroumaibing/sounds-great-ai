// Package threads holds the domain model for conversation threads. This file
// implements the message / delivery / branch substrate (README 十大缺口 #1):
// a delivery_status state machine, message_id idempotency (de-duplication),
// soft-delete and branch (edit-copy) modeling.
//
// The Inbox is an in-memory, goroutine-safe read-model. It is intentionally
// storage-agnostic: persistence (if any) is layered by the platform via the
// thread ports; this package owns only the invariants.
package threads

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// DeliveryStatus is the lifecycle state of a delivered message.
type DeliveryStatus string

const (
	// DeliveryStatusQueued means the message has been received but not yet
	// handed to its destination.
	DeliveryStatusQueued DeliveryStatus = "queued"
	// DeliveryStatusDelivered means the message was handed to its destination.
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	// DeliveryStatusCanceled means delivery was aborted and the message must
	// NEVER be delivered again (F185 fairness / no duplicate delivery).
	DeliveryStatusCanceled DeliveryStatus = "canceled"
)

// deliveryTransitions encodes the legal state machine:
//
//	queued ──▶ delivered
//	  │
//	  └──────▶ canceled
//
// delivered and canceled are terminal: a message that has left `queued` can
// never change status again. This is what guarantees "canceled messages are
// never re-delivered".
var deliveryTransitions = map[DeliveryStatus][]DeliveryStatus{
	DeliveryStatusQueued:    {DeliveryStatusDelivered, DeliveryStatusCanceled},
	DeliveryStatusDelivered: {},
	DeliveryStatusCanceled:  {},
}

// IsValidDeliveryStatus reports whether s is a known status.
func IsValidDeliveryStatus(s DeliveryStatus) bool {
	_, ok := deliveryTransitions[s]
	return ok
}

// CanTransition reports whether moving from->to is a legal transition.
func CanTransition(from, to DeliveryStatus) bool {
	for _, allowed := range deliveryTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// TransitionDeliveryStatus validates a state-machine move and returns an error
// for illegal transitions. Terminal states (delivered, canceled) reject every
// transition, including back to queued.
func TransitionDeliveryStatus(from, to DeliveryStatus) error {
	if !IsValidDeliveryStatus(from) {
		return fmt.Errorf("unknown delivery status %q", from)
	}
	if !IsValidDeliveryStatus(to) {
		return fmt.Errorf("unknown delivery status %q", to)
	}
	if from == to {
		return nil
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("illegal delivery transition %q -> %q", from, to)
	}
	return nil
}

// InboxMessage is one message in a thread's inbox. It carries its own
// delivery state plus soft-delete / branch metadata.
type InboxMessage struct {
	// ID is the message_id. It is the dedup key: Receiving the same ID twice
	// is a no-op (idempotency).
	ID string `json:"id"`
	// ThreadID groups messages.
	ThreadID string `json:"thread_id"`
	// Sender identifies the author (user / agent / connector / mcp).
	Sender string `json:"sender"`
	// Content is the opaque payload.
	Content string `json:"content"`
	// DeliveryStatus is the current state-machine value.
	DeliveryStatus DeliveryStatus `json:"delivery_status"`
	// BranchOf, when non-empty, marks this message as an EDIT COPY of the
	// message with that ID. The original is soft-deleted when a branch is cut,
	// modeling "同一消息的编辑副本".
	BranchOf string `json:"branch_of,omitempty"`
	// SoftDeleted marks a message as logically removed (never physically
	// deleted from the read-model; excluded from normal listings).
	SoftDeleted bool `json:"soft_deleted"`
	// ReceivedAt is the first time this message_id was seen.
	ReceivedAt time.Time `json:"received_at"`
}

// Inbox is a goroutine-safe, in-memory message inbox keyed by message_id.
type Inbox struct {
	mu       sync.RWMutex
	messages map[string]*InboxMessage
}

// NewInbox creates an empty inbox.
func NewInbox() *Inbox {
	return &Inbox{messages: make(map[string]*InboxMessage)}
}

// Receive inserts msg. It is idempotent: if msg.ID already exists the call is a
// no-op and (true, nil) is returned (the message was already present). On first
// receipt the message starts in DeliveryStatusQueued. Errors: empty ID, invalid
// delivery status, or an illegal pre-set status.
func (in *Inbox) Receive(msg InboxMessage) (bool, error) {
	if msg.ID == "" {
		return false, errors.New("inbox: message_id is required")
	}
	if msg.DeliveryStatus == "" {
		msg.DeliveryStatus = DeliveryStatusQueued
	}
	if !IsValidDeliveryStatus(msg.DeliveryStatus) {
		return false, fmt.Errorf("inbox: invalid delivery status %q", msg.DeliveryStatus)
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if existing, ok := in.messages[msg.ID]; ok {
		// Idempotent: return the canonical record, do NOT mutate it.
		_ = existing
		return true, nil
	}
	now := time.Now()
	stored := msg
	stored.ReceivedAt = now
	in.messages[msg.ID] = &stored
	return false, nil
}

// Get returns a copy of the message and whether it exists.
func (in *Inbox) Get(messageID string) (InboxMessage, bool) {
	in.mu.RLock()
	defer in.mu.RUnlock()
	m, ok := in.messages[messageID]
	if !ok {
		return InboxMessage{}, false
	}
	return *m, true
}

// SetDeliveryStatus transitions the message's delivery state, enforcing the
// state machine. This is the single choke point for delivery, so the invariant
// "canceled messages never become delivered" holds by construction.
func (in *Inbox) SetDeliveryStatus(messageID string, to DeliveryStatus) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	m, ok := in.messages[messageID]
	if !ok {
		return fmt.Errorf("inbox: unknown message_id %q", messageID)
	}
	if err := TransitionDeliveryStatus(m.DeliveryStatus, to); err != nil {
		return err
	}
	m.DeliveryStatus = to
	return nil
}

// SoftDelete marks a message as logically removed. Delivered/canceled messages
// may still be soft-deleted (e.g. user retracts a delivered message); this only
// affects listing visibility.
func (in *Inbox) SoftDelete(messageID string) error {
	in.mu.Lock()
	defer in.mu.Unlock()
	m, ok := in.messages[messageID]
	if !ok {
		return fmt.Errorf("inbox: unknown message_id %q", messageID)
	}
	m.SoftDeleted = true
	return nil
}

// Branch cuts an edit copy of originalID. The returned message carries a fresh
// message_id, points at the original via BranchOf, and the original is
// soft-deleted (so listings show the latest edit copy, never the stale one).
// Errors if the original is unknown or already soft-deleted.
func (in *Inbox) Branch(originalID string, branch InboxMessage) (*InboxMessage, error) {
	if branch.ID == "" {
		return nil, errors.New("inbox: branch message_id is required")
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	orig, ok := in.messages[originalID]
	if !ok {
		return nil, fmt.Errorf("inbox: unknown original message_id %q", originalID)
	}
	if orig.SoftDeleted {
		return nil, fmt.Errorf("inbox: cannot branch a soft-deleted message %q", originalID)
	}
	if _, dup := in.messages[branch.ID]; dup {
		return nil, fmt.Errorf("inbox: branch message_id %q already exists", branch.ID)
	}
	orig.SoftDeleted = true
	now := time.Now()
	stored := branch
	if stored.DeliveryStatus == "" {
		stored.DeliveryStatus = DeliveryStatusQueued
	}
	stored.BranchOf = originalID
	stored.ThreadID = orig.ThreadID
	stored.ReceivedAt = now
	in.messages[branch.ID] = &stored
	return &stored, nil
}

// MessagesForThread returns the messages of a thread. When includeSoftDeleted is
// false, soft-deleted (and their replaced branches' originals) are excluded.
func (in *Inbox) MessagesForThread(threadID string, includeSoftDeleted bool) []InboxMessage {
	in.mu.RLock()
	defer in.mu.RUnlock()
	out := make([]InboxMessage, 0, len(in.messages))
	for _, m := range in.messages {
		if m.ThreadID != threadID {
			continue
		}
		if !includeSoftDeleted && m.SoftDeleted {
			continue
		}
		out = append(out, *m)
	}
	return out
}
