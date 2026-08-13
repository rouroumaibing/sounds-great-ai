package services

import (
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// guardError carries the specific rejected reason for the audit-only
// ball.disposition_rejected event, mirroring clowder-ai's three-piece
// assertion failures (assertCurrentHolder / assertLatestInvocation /
// assertExactHandoffIsLive).
type guardError struct{ reason string }

func (e *guardError) Error() string { return "custody guard: " + e.reason }

// guardReason extracts the rejection reason for the audit event payload.
func guardReason(err error) string {
	if ge, ok := err.(*guardError); ok {
		return ge.reason
	}
	return "guard"
}

// assertCurrentHolder fails when `from` is no longer the projected holder
// (clowder assertCurrentHolder). A newer handoff/hold has superseded it.
func assertCurrentHolder(snap custodyPorts.BallSnapshot, from string) error {
	if snap.Holder != from {
		return &guardError{"holder"}
	}
	return nil
}

// assertLatestInvocation fails when a newer invocation (started by a different
// breed) has since taken the ball (clowder assertLatestInvocation). A stale
// invocation's disposition callback must not converge a live ball.
func assertLatestInvocation(events []custodyPorts.BallEvent, from string) error {
	latest := ""
	for _, e := range events {
		if e.Type == custodyPorts.InvocationStarted {
			latest = e.Holder
		}
	}
	if latest != "" && latest != from {
		return &guardError{"invocation"}
	}
	return nil
}

// assertExactHandoffIsLive fails when the tip of the custody log is a handoff
// from a different breed than `from` (clowder assertExactHandoffIsLive). The
// specific handoff being closed must still be the live tip, not one that was
// superseded by a newer handoff. `to` is "" for hold dispositions (no target).
func assertExactHandoffIsLive(events []custodyPorts.BallEvent, from, to string) error {
	lastFrom := ""
	for _, e := range events {
		if e.Type == custodyPorts.BallHanded || e.Type == custodyPorts.DispatchDispositioned {
			lastFrom = e.From
		}
	}
	if lastFrom != "" && lastFrom != from {
		return &guardError{"handoff"}
	}
	return nil
}
