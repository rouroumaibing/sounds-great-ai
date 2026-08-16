package services

import (
	custodyPorts "sounds-great-ai/internal/domains/custody/ports"
)

// guardError carries the specific rejected reason for the audit-only
// ball.disposition_rejected event, mirroring the three-piece
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

// assertCurrentHolder fails when `from` is no longer the projected holder.
// A newer handoff/hold has superseded it.
func assertCurrentHolder(snap custodyPorts.BallSnapshot, from string) error {
	if snap.Holder != from {
		return &guardError{"holder"}
	}
	return nil
}

// assertLatestInvocation fails when a newer invocation (started by a different
// breed) has since taken the ball. A stale
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

// assertExactHandoffIsLive fails when the disposer `from` is NOT the receiver of
// the current live handoff. The specific
// handoff being closed must still be the live tip: the most recent handoff's
// `To` must equal `from`. A breed that never received the live handoff (e.g. a
// superseded or duplicate sender) cannot dispose it. `to` is unused here (the
// new target is chosen by this disposition).
func assertExactHandoffIsLive(events []custodyPorts.BallEvent, from, _ string) error {
	lastTo := ""
	for _, e := range events {
		if e.Type == custodyPorts.BallHanded || e.Type == custodyPorts.DispatchDispositioned {
			lastTo = e.To
		}
	}
	if lastTo != "" && lastTo != from {
		return &guardError{"handoff"}
	}
	return nil
}
