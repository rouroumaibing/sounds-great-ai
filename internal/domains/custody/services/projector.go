package services

import (
	"sort"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// Project is a pure function: it folds an ordered event log into the current
// ball custody snapshot. It performs no IO and is fully deterministic, mirroring
// the ball-custody-state-machine transition() pure function.
func Project(events []custodyPorts.BallEvent) custodyPorts.BallSnapshot {
	snap := custodyPorts.BallSnapshot{State: custodyPorts.BallStateNew}

	// Defensive ordering by Seq (store normally returns append order).
	ordered := make([]custodyPorts.BallEvent, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Seq < ordered[j].Seq })

	for _, e := range ordered {
		switch e.Type {
		case custodyPorts.BallHanded, custodyPorts.DispatchDispositioned:
			snap.State = custodyPorts.BallStateActive
			if e.To != "" {
				snap.Holder = e.To
			}
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
			if e.Generation > snap.Generation {
				snap.Generation = e.Generation
			}
		case custodyPorts.InvocationStarted:
			snap.State = custodyPorts.BallStateActive
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.InvocationHeartbeat:
			if snap.State == custodyPorts.BallStateActive {
				if e.Holder != "" {
					snap.Holder = e.Holder
				}
			}
		case custodyPorts.BallHeld:
			snap.State = custodyPorts.BallStateParked
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.WakeConditionMet:
			// Intermediate event: the wake condition fired but disposition has
			// not committed yet. The thread stays parked until hold_dispositioned
			// flips it back to active. Kept distinct from hold_dispositioned so
			// the audit trail shows exactly when the external condition was met.
			if snap.State == custodyPorts.BallStateParked {
				if e.Holder != "" {
					snap.Holder = e.Holder
				}
			}
		case custodyPorts.HoldDispositioned:
			snap.State = custodyPorts.BallStateActive
		case custodyPorts.InvocationDied:
			snap.State = custodyPorts.BallStateZombie
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.TaskBlocked:
			snap.State = custodyPorts.BallStateBlocked
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.TaskDone:
			snap.State = custodyPorts.BallStateResolved
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		// G4: add-only lifecycle events. Each pins a deterministic state effect.
		case custodyPorts.BallDispositionRejected:
			// Audit-only: a rejected disposition must never mutate state.
		case custodyPorts.BallHandedCVO:
			// Escalated to operator/CVO: the ball parks awaiting a human.
			snap.State = custodyPorts.BallStateParked
			if e.Holder != "" {
				snap.Holder = e.Holder
			} else if e.To != "" {
				snap.Holder = e.To
			}
		case custodyPorts.BallVoidPass:
			// A handoff with no valid target: the pass is voided.
			snap.State = custodyPorts.BallStateVoid
		case custodyPorts.BallHoldExpired:
			// The hold's deadline elapsed without a wake: the ball dies.
			snap.State = custodyPorts.BallStateDead
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.BallWakeSent:
			// External wake signal emitted: intermediate, no state change.
		case custodyPorts.BallFrozen:
			// Coordination frozen pending review: treat as a hard stop.
			snap.State = custodyPorts.BallStateBlocked
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.BallDegraded:
			// Heartbeat late but within grace: still active, flagged degraded.
			if snap.State == custodyPorts.BallStateActive || snap.State == custodyPorts.BallStateNew {
				snap.State = custodyPorts.BallStateActive
			}
			snap.Degraded = true
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.BallAbandoned:
			// Task abandoned: the ball dies.
			snap.State = custodyPorts.BallStateDead
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.TaskUnblocked:
			// A blocked ball was explicitly unblocked back to active.
			if snap.State == custodyPorts.BallStateBlocked {
				snap.State = custodyPorts.BallStateActive
			}
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		case custodyPorts.TaskIdleLong:
			// Parked/idle too long without progress: mark blocked for attention.
			snap.State = custodyPorts.BallStateBlocked
			if e.Holder != "" {
				snap.Holder = e.Holder
			}
		}
		snap.ThreadID = e.ThreadID
		if e.Timestamp > snap.UpdatedAt {
			snap.UpdatedAt = e.Timestamp
		}
	}
	return snap
}
