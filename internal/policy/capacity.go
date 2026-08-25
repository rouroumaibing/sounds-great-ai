package policy

import (
	"errors"
	"fmt"
)

// CapacitySnapshot is a point-in-time view of a session's resource usage.
type CapacitySnapshot struct {
	// Used is the consumed amount within the window (e.g. tokens, messages).
	Used int64
	// Limit is the cap for the window. A non-positive limit is treated as
	// "ungoverned" and makes EvaluateSealGate reject (fail-closed) rather than
	// silently admit everything.
	Limit int64
	// Window labels the budgeting window (e.g. "daily", "per-session").
	Window string
	// Pinned reports whether the session is pinned (exempt from auto-seal).
	Pinned bool
}

// UsageRatio returns Used/Limit in [0,1+]. Returns 0 when Limit<=0 (ungoverned
// is reported as zero ratio so callers must consult Exceeds/Err explicitly).
func (s CapacitySnapshot) UsageRatio() float64 {
	if s.Limit <= 0 {
		return 0
	}
	return float64(s.Used) / float64(s.Limit)
}

// Exceeds reports whether usage has reached or passed the limit.
func (s CapacitySnapshot) Exceeds() bool {
	return s.Limit > 0 && s.Used >= s.Limit
}

// WarningLevel is the severity of a usage warning.
type WarningLevel string

const (
	WarnInfo     WarningLevel = "info"
	WarnWarning  WarningLevel = "warning"
	WarnCritical WarningLevel = "critical"
)

// UsageWarning is an emitted capacity event.
type UsageWarning struct {
	Level WarningLevel
	Ratio float64
	Used  int64
	Limit int64
	// ShouldSeal is true when the gate decided a seal is required.
	ShouldSeal bool
	Message    string
}

// ErrCapacityUngoverned is returned by EvaluateSealGate when Limit<=0: an
// unconfigured capacity must never auto-admit (fail-closed).
var ErrCapacityUngoverned = errors.New("policy: capacity limit unset (fail-closed)")

// EvaluateSealGate decides whether a session must be sealed at the given
// threshold (0..1) and emits the appropriate warning.
//
// Fail-closed: an ungoverned snapshot (Limit<=0) returns an error and MUST be
// treated as "seal now" by the caller. A pinned session is never sealed but
// still emits a critical warning so operators see the pressure.
func EvaluateSealGate(snap CapacitySnapshot, threshold float64) (bool, UsageWarning, error) {
	if snap.Limit <= 0 {
		return true, UsageWarning{
			Level:      WarnCritical,
			ShouldSeal: true,
			Used:       snap.Used,
			Limit:      snap.Limit,
			Message:    "capacity limit unset: sealing to avoid ungoverned admission",
		}, ErrCapacityUngoverned
	}
	ratio := snap.UsageRatio()
	w := UsageWarning{Used: snap.Used, Limit: snap.Limit, Ratio: ratio}

	if snap.Exceeds() {
		w.Level = WarnCritical
		w.ShouldSeal = true
		w.Message = fmt.Sprintf("capacity exceeded: %d/%d (%.0f%%)", snap.Used, snap.Limit, ratio*100)
		return true, w, nil
	}
	if ratio >= threshold {
		w.Level = WarnCritical
		w.ShouldSeal = !snap.Pinned
		w.Message = fmt.Sprintf("capacity at seal threshold: %.0f%%", ratio*100)
		return w.ShouldSeal, w, nil
	}
	if ratio >= threshold*0.8 {
		w.Level = WarnWarning
		w.Message = fmt.Sprintf("capacity approaching seal: %.0f%%", ratio*100)
		return false, w, nil
	}
	w.Level = WarnInfo
	w.Message = fmt.Sprintf("capacity nominal: %.0f%%", ratio*100)
	return false, w, nil
}

// SessionPin records a pinned (seal-exempt) session. Pinning requires an
// explicit reason and a bound principal id so it is auditable and revocable.
type SessionPin struct {
	SessionID string
	Principal string
	Reason    string
	PinnedAt  int64 // unix millis
}

// Pin returns a CapacitySnapshot copy with Pinned=true.
func (s CapacitySnapshot) Pin(p SessionPin) CapacitySnapshot {
	s.Pinned = true
	return s
}
