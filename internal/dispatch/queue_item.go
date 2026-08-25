// Package dispatch implements the invocation scheduling substrate for the
// multi-dog agent platform: a user-priority queue with urgent marking, user
// batching/serialization, auto-execution, and connector fairness under A2A
// saturation (F175 priority queue, F185 busy-layering / fairness).
//
// Key invariants (each covered by a unit test, see *_test.go):
//
//  1. Urgent messages do NOT bypass the queue. "urgent" is modeled purely as a
//     top priority value; urgent items still flow through Enqueue -> Dequeue.
//     There is deliberately NO fast-path method that executes an item without
//     first dequeuing it.
//  2. Connector calls are not starved when the A2A channel is saturated. The
//     Dequeue policy promotes a pending connector call once agent items served
//     since the last connector exceed a starve limit.
//  3. Canceled messages are never re-delivered (enforced in the threads.Inbox
//     state machine; the queue only ever Dequeues an item once).
package dispatch

import (
	"time"
)

// ItemKind classifies a queued invocation. Agent items are A2A agent
// invocations (the saturatable channel); connector items are external connector
// calls that must not be starved; user items are end-user messages.
type ItemKind string

const (
	ItemKindUser      ItemKind = "user"
	ItemKindAgent     ItemKind = "agent" // A2A agent invocation (saturatable)
	ItemKindConnector ItemKind = "connector"
)

// QueueItem is one schedulable invocation.
type QueueItem struct {
	// ID uniquely identifies this queued item. Assigned by Enqueue if empty.
	ID string
	// ThreadID groups the invocation to a conversation thread.
	ThreadID string
	// User identifies the owning user (for batching/serialization).
	User string
	// Kind classifies the item for fairness scheduling.
	Kind ItemKind
	// Priority is the user-supplied priority (higher = sooner). Urgent items
	// are stored with the maximum effective priority but still go through the
	// queue (invariant #1).
	Priority int
	// Urgent marks the item as top-priority. It does NOT skip Enqueue/Dequeue.
	Urgent bool
	// AutoExec marks items that the scheduler may attempt to execute via
	// TryAutoExecute.
	AutoExec bool
	// Payload is opaque caller data.
	Payload any
	// enqueuedAt records arrival order for FIFO tie-breaking.
	enqueuedAt time.Time
	// seq is a monotonic insertion sequence for stable ordering.
	seq int64
}

// effectivePriority returns the priority used for ordering. Urgent collapses to
// the maximum int, but the item is still ordered/selected through Dequeue like
// any other — there is no separate execution path.
func (qi QueueItem) effectivePriority() int {
	if qi.Urgent {
		return int(^uint(0) >> 1) // math.MaxInt
	}
	return qi.Priority
}
