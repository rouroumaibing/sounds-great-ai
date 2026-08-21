package ports

// SubstantiveActivity describes whether a handoff carried real work, used by the
// worklist ping-pong breaker (G2). Mirrors the isSubstantiveActivity rule:
// a long output or a real tool call breaks the "language inertia" that the
// breaker is designed to stop.
type SubstantiveActivity struct {
	// OutputLen is the rune length of the handing-off dog's output.
	OutputLen int
	// HadToolCall is true when the dog performed a real tool call (vs. a short
	// conversational reply). SG does not yet surface per-invocation tool calls,
	// so callers set it from available signals (default false).
	HadToolCall bool
}

// Ping-pong breaker thresholds (G2). Hardcoded per "Hardcoded
// per KD (YAGNI — no config)"): warn at 2 consecutive same-pair handoffs, block
// (terminate the chain) at 4. A block still allows 3 normal back-and-forths.
const (
	PingPongWarnThreshold  = 2
	PingPongBlockThreshold = 4
	// SubstantiveOutputLenT is the output-length cutoff (runes) above which a
	// handoff is treated as substantive work and resets the streak
	// (OUTPUT_LEN_T = 200).
	SubstantiveOutputLenT = 200
)

// IWorklist is the per-invocation A2A worklist (G2). It tracks chain depth and
// the consecutive same-pair (A↔B) handoff streak so the orchestrator can break
// ping-pong loops and enforce a per-message depth ceiling before recursing into
// another dog. Register is called once per user message (invocation); Push is
// called before each handoff.
type IWorklist interface {
	// Register creates (or no-ops over an existing) worklist for invID with the
	// given max chain depth. Safe to call on every top-level dispatch.
	Register(invID string, maxDepth int)
	// Push consults the depth and ping-pong guards for a handoff from→to.
	// accepted=false means the handoff must NOT proceed (reason: "depth" |
	// "pingpong"). warn=true means the handoff may proceed but the next dog
	// should receive a ping-pong warning.
	Push(invID, from, to string, activity SubstantiveActivity) (accepted bool, reason string, warn bool)
	// Done releases the worklist state for invID (called after the invocation
	// and all its recursive handoffs complete).
	Done(invID string)
	// PushToWorklist dynamically appends new targets to a running worklist at
	// runtime (G11: pushToWorklist fan-out). It returns the subset of
	// targets actually added (new, not already present, within the fan-out cap),
	// recording each addition's fromRef for attribution and dedup. A target that
	// has already been executed is not re-added.
	PushToWorklist(invID string, targets []string, fromRef string) []string
}
