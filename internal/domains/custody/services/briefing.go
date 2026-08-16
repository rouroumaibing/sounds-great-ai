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

// ProjectUnifiedTrail folds the custody trail and the code-repo git-ref
// trajectory into a single time-ordered axis (G14). Repo events are filtered to
// the thread's [first, last] custody timestamp window so only code activity
	// around the collaboration surfaces — mirroring the feat-trajectory
	// merge (git + feat + thread on one per-feature axis). When repo is empty (no
// configured repo URL) the unified list equals the custody trail.
func (l *BallLedger) ProjectUnifiedTrail(ctx context.Context, threadID string, repo []stores.RepoEvent) ([]custodyPorts.UnifiedTrailEntry, error) {
	b, err := l.ProjectTrail(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return MergeUnifiedTrail(b, repo), nil
}

// MergeUnifiedTrail is the pure projection behind ProjectUnifiedTrail. It is
// exported so the transport layer can compose the unified timeline directly
// (the custody ledger is consumed as an interface that must not import stores,
// to avoid an import cycle).
func MergeUnifiedTrail(b custodyPorts.Briefing, repo []stores.RepoEvent) []custodyPorts.UnifiedTrailEntry {
	out := make([]custodyPorts.UnifiedTrailEntry, 0, len(b.Trail)+len(repo))
	var minTs, maxTs int64
	for i, t := range b.Trail {
		out = append(out, custodyPorts.UnifiedTrailEntry{
			Source:    "custody",
			Timestamp: t.Timestamp,
			Kind:      t.Type,
			Holder:    t.Holder,
			From:      t.From,
			To:        t.To,
			Seq:       t.Seq,
		})
		if i == 0 || t.Timestamp < minTs {
			minTs = t.Timestamp
		}
		if t.Timestamp > maxTs {
			maxTs = t.Timestamp
		}
	}
	for _, r := range repo {
		if (minTs > 0 && r.At < minTs) || (maxTs > 0 && r.At > maxTs) {
			continue
		}
		out = append(out, custodyPorts.UnifiedTrailEntry{
			Source:    "repo",
			Timestamp: r.At,
			Kind:      r.Kind,
			Branch:    r.Branch,
			HeadSHA:   r.HeadSHA,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out
}
