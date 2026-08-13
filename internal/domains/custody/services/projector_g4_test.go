package services

import (
	"testing"

	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// mustProject folds a single event list and returns the snapshot.
func mustProject(events []custodyPorts.BallEvent) custodyPorts.BallSnapshot {
	return Project(events)
}

// G4: each add-only lifecycle event pins a deterministic projection effect.
func TestProjectG4LifecycleEvents(t *testing.T) {
	cases := []struct {
		name string
		evs  []custodyPorts.BallEvent
		state custodyPorts.BallState
		holder string
		degraded bool
	}{
		{
			name:  "void_pass -> void",
			evs:   []custodyPorts.BallEvent{{Type: custodyPorts.BallVoidPass}},
			state: custodyPorts.BallStateVoid,
		},
		{
			name:  "hold_expired -> dead",
			evs:   []custodyPorts.BallEvent{{Type: custodyPorts.BallHoldExpired}},
			state: custodyPorts.BallStateDead,
		},
		{
			name:   "handed_cvo -> parked (operator)",
			evs:    []custodyPorts.BallEvent{{Type: custodyPorts.BallHandedCVO, Holder: "cxo"}},
			state:  custodyPorts.BallStateParked,
			holder: "cxo",
		},
		{
			name:   "frozen -> blocked",
			evs:    []custodyPorts.BallEvent{{Type: custodyPorts.BallFrozen, Holder: "x"}},
			state:  custodyPorts.BallStateBlocked,
			holder: "x",
		},
		{
			name:     "degraded (from new) -> active + flag",
			evs:      []custodyPorts.BallEvent{{Type: custodyPorts.BallDegraded}},
			state:    custodyPorts.BallStateActive,
			degraded: true,
		},
		{
			name:     "degraded (after active) -> active + flag, holder kept",
			evs:      []custodyPorts.BallEvent{{Type: custodyPorts.BallHanded, To: "a"}, {Type: custodyPorts.BallDegraded}},
			state:    custodyPorts.BallStateActive,
			holder:   "a",
			degraded: true,
		},
		{
			name:  "abandoned -> dead",
			evs:   []custodyPorts.BallEvent{{Type: custodyPorts.BallAbandoned}},
			state: custodyPorts.BallStateDead,
		},
		{
			name:  "unblocked -> active",
			evs:   []custodyPorts.BallEvent{{Type: custodyPorts.TaskBlocked}, {Type: custodyPorts.TaskUnblocked}},
			state: custodyPorts.BallStateActive,
		},
		{
			name:  "idle_long (after blocked) -> blocked",
			evs:   []custodyPorts.BallEvent{{Type: custodyPorts.TaskBlocked}, {Type: custodyPorts.TaskIdleLong}},
			state: custodyPorts.BallStateBlocked,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			snap := mustProject(c.evs)
			if snap.State != c.state {
				t.Fatalf("state = %s, want %s", snap.State, c.state)
			}
			if c.holder != "" && snap.Holder != c.holder {
				t.Fatalf("holder = %s, want %s", snap.Holder, c.holder)
			}
			if snap.Degraded != c.degraded {
				t.Fatalf("degraded = %v, want %v", snap.Degraded, c.degraded)
			}
		})
	}
}

// G4: ball.wake_sent and ball.disposition_rejected must NEVER mutate state.
func TestProjectG4NoStateMutationEvents(t *testing.T) {
	// wake_sent between held and dispositioned stays parked.
	snap := mustProject([]custodyPorts.BallEvent{
		{Type: custodyPorts.BallHeld, Holder: "a"},
		{Type: custodyPorts.BallWakeSent, Holder: "a"},
	})
	if snap.State != custodyPorts.BallStateParked || snap.Holder != "a" {
		t.Fatalf("wake_sent mutated state: %+v", snap)
	}

	// disposition_rejected on an active ball must not change holder/state.
	snap = mustProject([]custodyPorts.BallEvent{
		{Type: custodyPorts.BallHanded, To: "b"},
		{Type: custodyPorts.BallDispositionRejected},
	})
	if snap.State != custodyPorts.BallStateActive || snap.Holder != "b" {
		t.Fatalf("disposition_rejected mutated state: %+v", snap)
	}
}
