package services

import (
	"context"
	"sort"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
	"sounds-great-ai/internal/domains/custody/stores"
)

// ProjectTrail folds the ledger events for threadID into a Briefing. It is a
// read-only projection (the engine behind the Brief & Trail API) and never
// mutates the ledger.
func (l *BallLedger) ProjectTrail(ctx context.Context, threadID string) (custodyPorts.Briefing, error) {
	events, err := l.store.GetEvents(ctx, threadID)
	if err != nil {
		return custodyPorts.Briefing{}, err
	}
	snap := Project(events)

	var turns, handoffs, holds int
	trail := make([]custodyPorts.TrailEntry, 0, len(events))
	for _, e := range events {
		trail = append(trail, custodyPorts.TrailEntry{
			Seq:       e.Seq,
			Type:      string(e.Type),
			Holder:    e.Holder,
			From:      e.From,
			To:        e.To,
			Timestamp: e.Timestamp,
		})
		switch e.Type {
		case custodyPorts.BallHanded, custodyPorts.InvocationStarted:
			turns++
		case custodyPorts.DispatchDispositioned:
			handoffs++
		case custodyPorts.BallHeld:
			holds++
		}
	}

	return custodyPorts.Briefing{
		ThreadID: threadID,
		State:    string(snap.State),
		Holder:   snap.Holder,
		Turns:    turns,
		Handoffs: handoffs,
		Holds:    holds,
		Trail:    trail,
	}, nil
}
