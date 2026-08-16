package unified

import "context"

// CarrierHealthEvent is the structured payload broadcast to clients (T25 / R6)
// when a carrier's health changes or a transport tier is skipped during
// fallback. It lets the frontend ConnectionStatusBar render per-carrier
// upstream health instead of inferring it from raw stream events.
type CarrierHealthEvent struct {
	Carrier    string `json:"carrier"` // e.g. "claude" or "claude/bg_daemon"
	Transport  string `json:"transport,omitempty"`
	Level      string `json:"level"` // "online" | "degraded" | "offline"
	Reason     string `json:"reason,omitempty"`
	RemainingMs int64 `json:"remaining_ms,omitempty"`
}

// HealthBroadcaster pushes carrier-health changes to interested clients
// (the WebSocket hub). The unified layer defines this interface so it has no
// hard dependency on the transport package (which would be an import cycle:
// transport imports platform, platform imports unified).
type HealthBroadcaster interface {
	BroadcastCarrierHealth(ctx context.Context, ev CarrierHealthEvent)
}

// NoopBroadcaster is the default HealthBroadcaster (no WebSocket wired).
// The platform layer swaps in a real broadcaster at startup (T25).
type NoopBroadcaster struct{}

// BroadcastCarrierHealth is a no-op.
func (NoopBroadcaster) BroadcastCarrierHealth(_ context.Context, _ CarrierHealthEvent) {}
