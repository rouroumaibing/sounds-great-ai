package dispatch

import (
	"context"
	"errors"
	"sync"
	"time"
)

// defaultConnectorStarveLimit is how many agent items may be served after the
// last connector before the scheduler promotes a pending connector call (even
// under A2A saturation). It bounds connector starvation.
const defaultConnectorStarveLimit = 4

// InvocationQueue is a thread-safe priority queue of invocations honoring the
// invariants documented on the package. It is in-memory and storage-agnostic.
type InvocationQueue struct {
	mu sync.Mutex

	items []QueueItem
	seq   int64

	// a2aBusy models A2A channel saturation. While true, agent items are still
	// served but the connector-fairness promotion activates.
	a2aBusy bool
	// agentServedSinceConnector counts agent items dequeued since the last
	// connector was served; used to trigger fairness promotion.
	agentServedSinceConnector int
	// connectorStarveLimit caps agent items between connector servings.
	connectorStarveLimit int

	// batching, when true, serializes same-user user-items: a newly enqueued
	// user item is positioned immediately after the last queued user item of
	// the same user so they execute consecutively (F185 user batching).
	batching bool
}

// NewInvocationQueue creates an empty queue with sensible defaults.
func NewInvocationQueue() *InvocationQueue {
	return &InvocationQueue{
		connectorStarveLimit: defaultConnectorStarveLimit,
		batching:             true,
	}
}

// SetA2ASaturation toggles the A2A-busy signal. This is the external hint the
// scheduler uses to decide whether connector fairness promotion is active.
func (q *InvocationQueue) SetA2ASaturation(busy bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.a2aBusy = busy
}

// SetBatching enables/disables same-user serialization.
func (q *InvocationQueue) SetBatching(on bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.batching = on
}

// Len returns the number of queued items.
func (q *InvocationQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Enqueue inserts item. This is the ONLY ingest path — urgent and connector
// items use it exactly like any other, guaranteeing invariant #1 (no bypass).
// A missing ID is assigned; Urgent is preserved but only affects ordering.
func (q *InvocationQueue) Enqueue(item QueueItem) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.seq++
	if item.ID == "" {
		item.ID = newItemID(q.seq)
	}
	item.seq = q.seq
	item.enqueuedAt = time.Now()

	if q.batching && item.Kind == ItemKindUser && item.User != "" {
		// Serialize: insert right after the last queued user item of same user.
		insertAt := len(q.items)
		for i := len(q.items) - 1; i >= 0; i-- {
			if q.items[i].Kind == ItemKindUser && q.items[i].User == item.User {
				insertAt = i + 1
				break
			}
		}
		q.items = append(q.items, QueueItem{})
		copy(q.items[insertAt+1:], q.items[insertAt:])
		q.items[insertAt] = item
		return item.ID
	}

	q.items = append(q.items, item)
	return item.ID
}

// Dequeue removes and returns the next item according to the scheduling policy.
// Returns (zero, false) when empty.
func (q *InvocationQueue) Dequeue() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	idx := q.selectIndexLocked()
	item := q.items[idx]
	q.items = append(q.items[:idx], q.items[idx+1:]...)

	// Update fairness counters.
	switch item.Kind {
	case ItemKindConnector:
		q.agentServedSinceConnector = 0
	case ItemKindAgent:
		q.agentServedSinceConnector++
	}
	return item, true
}

// Peek returns the next item without removing it.
func (q *InvocationQueue) Peek() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	return q.items[q.selectIndexLocked()], true
}

// selectIndexLocked picks the index to dequeue. Caller must hold mu.
//
// Policy (invariant #1 + #2):
//   - If A2A is busy AND a connector item is pending AND connectors have been
//     starved beyond the limit, promote the highest-priority connector item.
//   - Otherwise pick the highest effective-priority item (urgent wins via
//     effectivePriority), FIFO tie-break on insertion sequence.
func (q *InvocationQueue) selectIndexLocked() int {
	promoteConnector := q.a2aBusy &&
		q.agentServedSinceConnector >= q.connectorStarveLimit &&
		q.hasKindLocked(ItemKindConnector)

	best := 0
	for i := 1; i < len(q.items); i++ {
		if q.betterLocked(i, best, promoteConnector) {
			best = i
		}
	}
	return best
}

// betterLocked reports whether candidate should be chosen over current, given
// the connector-promotion mode. Caller must hold mu.
func (q *InvocationQueue) betterLocked(candidate, current int, promoteConnector bool) bool {
	ci, cu := q.items[candidate], q.items[current]

	// In promotion mode, prefer connector items over non-connectors.
	if promoteConnector {
		cIsConn := ci.Kind == ItemKindConnector
		uIsConn := cu.Kind == ItemKindConnector
		if cIsConn != uIsConn {
			return cIsConn // connector wins
		}
	}

	cp, up := ci.effectivePriority(), cu.effectivePriority()
	if cp != up {
		return cp > up
	}
	// Stable FIFO: lower seq (earlier arrival) wins.
	return ci.seq < cu.seq
}

func (q *InvocationQueue) hasKindLocked(k ItemKind) bool {
	for _, it := range q.items {
		if it.Kind == k {
			return true
		}
	}
	return false
}

// TryAutoExecute attempts to execute the front item if it is auto-executable.
// It dequeues the item, runs fn under ctx, and returns (executed, err). When the
// front item is not AutoExec, it leaves the queue untouched and returns
// (false, nil). This is the ONLY auto-execution path and it still dequeues
// first, so nothing executes outside the queue lifecycle.
func (q *InvocationQueue) TryAutoExecute(ctx context.Context, fn func(QueueItem) error) (bool, error) {
	q.mu.Lock()
	if len(q.items) == 0 {
		q.mu.Unlock()
		return false, nil
	}
	idx := q.selectIndexLocked()
	if !q.items[idx].AutoExec {
		q.mu.Unlock()
		return false, nil
	}
	item := q.items[idx]
	q.items = append(q.items[:idx], q.items[idx+1:]...)
	switch item.Kind {
	case ItemKindConnector:
		q.agentServedSinceConnector = 0
	case ItemKindAgent:
		q.agentServedSinceConnector++
	}
	q.mu.Unlock()

	if err := fn(item); err != nil {
		return true, err
	}
	return true, nil
}

// HasQueuedNonAgentForThread reports whether threadID has a queued item that is
// NOT an agent (A2A) invocation. Used by callers to prevent agent work from
// starving end-user / connector messages on a thread (anti-starvation guard).
func (q *InvocationQueue) HasQueuedNonAgentForThread(threadID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, it := range q.items {
		if it.ThreadID == threadID && it.Kind != ItemKindAgent {
			return true
		}
	}
	return false
}

// ErrEmpty is returned by callers when an operation needs an item but none exist.
var ErrEmpty = errors.New("dispatch: queue is empty")

// newItemID builds a deterministic-enough id from the sequence. Caller holds mu
// only for seq uniqueness; collisions across restarts are not a concern for the
// in-memory read-model.
func newItemID(seq int64) string {
	return "inv-" + itoa(seq)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
