package unified

import (
	"context"
	"fmt"
)

// IsFatalMidStream reports whether the given diagnostic text corresponds to a
// fatal, carrier-switchable class of failure. Mirrors the
// FallbackCarrierWrapper classification: quota/rate-limit (429), auth,
// context-window, model-not-found, invalid-config, server-overloaded, and
// upstream policy rejections are permanent for the current carrier. Transient
// classes (network/overload/timeout) are retried by the caller, not switched
// here.
//
// NOTE: as of the R1 redesign, mid-stream fatal detection no
// longer triggers an in-invocation carrier swap (see RunCarrierFallback). This
// helper is retained for callers that need to classify a diagnostic string.
func IsFatalMidStream(text string) bool {
	switch ClassifyError(text) {
	case ReasonQuotaExceeded,
		ReasonAuthFailed,
		ReasonContextWindowExceeded,
		ReasonModelNotFound,
		ReasonInvalidConfig,
		ReasonServerOverloaded,
		ReasonUpstreamPolicyReject:
		return true
	}
	return false
}

// isTransientReason mirrors the classifyCarrierFailure default: transient
// (network/overload/timeout/stall/policy) does NOT degrade the carrier and is
// left to the caller's retry path. Everything else (quota/structural) degrades
// the tier and triggers a single fallback hop.
func isTransientReason(r ErrorReasonCode) bool {
	switch r {
	case ReasonNetworkError,
		ReasonServerOverloaded,
		ReasonResponseTimeout,
		ReasonStallTimeout,
		ReasonUpstreamPolicyReject:
		return true
	}
	return false
}

// healthKey builds the carrier-health key used by MemoryHealth/RedisHealth:
// "<carrierID>/<transportKind>".
func healthKey(carrierID string, kind TransportKind) string {
	return carrierID + "/" + string(kind)
}

// RunCarrierFallback streams spec through the carrier's transport chain with
// R1-aligned semantics (FallbackCarrierWrapper):
//
//   - THROWN spawn error: classify the error text.
//     · Transient (network/overload/timeout/stall/policy) → RecordFailure and
//     emit the error so the OUTER retry path handles it. No fallback.
//     · Non-transient (quota/structural) → RecordFailure(activeTier) so the
//     NEXT call skips this tier, then perform EXACTLY ONE fallback hop to the
//     next healthy tier (emitting a visible carrier_fallback system_info).
//     api_key is a no-op last resort → emit the original error instead of a
//     false fallback event. No cascading beyond the single hop.
//
//   - YIELDED error event (mid-stream): classify. Non-transient → RecordFailure
//     for the NEXT invocation. Crucially, SG does NOT mid-stream switch
//     here — partial output may already have been yielded to the user — so the
//     event is passed through unchanged. Transient yielded errors are passed
//     through with no action.
//
// The default single-tier chain ([print_sdk]) never falls back, preserving
// prior behavior; multi-tier chains (e.g. [bg_daemon, print_sdk]) gain the
// swap. A tier that cannot spawn is skipped; if every tier is exhausted without
// a clean stream, a terminal error event is emitted.
func RunCarrierFallback(
	ctx context.Context,
	reg *Registry,
	carrierID string,
	spec *SpawnSpec,
	streamFn func(*SpawnHandle) <-chan StreamEvent,
) (<-chan StreamEvent, error) {
	chain := reg.ResolveChain(carrierID)
	out := make(chan StreamEvent, 64)
	go func() {
		defer close(out)
		idx := 0
		// At most two attempts: the active tier, then exactly one fallback hop.
		for attempt := 0; attempt < 2; attempt++ {
			if idx >= len(chain) {
				break
			}
			tier := chain[idx]
			// Attempt exactly this one tier (ExecuteTier does not cascade), so a
			// failure here is attributable to `tier` and we can decide on a single
			// fallback hop rather than transparently drifting down the chain.
			h, err := reg.ExecuteTier(ctx, carrierID, spec, idx)
			if err != nil {
				cls := ClassifyError(err.Error())
				if isTransientReason(cls) {
					// Let the outer retry path handle transient failures.
					if reg.health != nil {
						reg.health.RecordFailure(ctx, healthKey(carrierID, tier), cls)
					}
					out <- StreamEvent{Type: "error", Content: err.Error()}
					return
				}
				// Quota/structural: record for NEXT call, then one fallback hop.
				if reg.health != nil {
					reg.health.RecordFailure(ctx, healthKey(carrierID, tier), cls)
				}
				nextIdx, ok := reg.nextHealthyTierIndex(ctx, carrierID, idx)
				if !ok {
					// No better tier (or only the api_key no-op remains) — surface
					// the original error rather than a false carrier_fallback event.
					out <- StreamEvent{Type: "error", Content: err.Error()}
					return
				}
				out <- StreamEvent{
					Type:    "system_info",
					Content: fmt.Sprintf("carrier fallback: %s -> %s", tier, chain[nextIdx]),
					Meta: map[string]any{
						"kind":    "carrier_fallback",
						"from":    string(tier),
						"to":      string(chain[nextIdx]),
						"carrier": carrierID,
					},
				}
				idx = nextIdx
				continue
			}
			// Stream the active tier, passing events through and recording any
			// yielded fatal error for the NEXT invocation (no mid-stream switch).
			monitorYieldedErrors(ctx, reg, carrierID, tier, h, streamFn, out)
			return
		}
		out <- StreamEvent{Type: "error", Content: fmt.Sprintf("all carriers failed for %q", carrierID)}
	}()
	return out, nil
}

// monitorYieldedErrors forwards the stream from streamFn(h) to out, classifying
// yielded error events. A non-transient yielded error is recorded against the
// current tier's health so the NEXT call skips it; crucially it is NOT used to
// switch carriers mid-stream (partial output may already be yielded).
func monitorYieldedErrors(
	ctx context.Context,
	reg *Registry,
	carrierID string,
	tier TransportKind,
	h *SpawnHandle,
	streamFn func(*SpawnHandle) <-chan StreamEvent,
	out chan<- StreamEvent,
) {
	for evt := range streamFn(h) {
		switch evt.Type {
		case "error":
			cls := ClassifyError(evt.Content)
			if !isTransientReason(cls) && reg.health != nil {
				reg.health.RecordFailure(ctx, healthKey(carrierID, tier), cls)
			}
		case "carrier_fatal":
			if reg.health != nil {
				reg.health.RecordFailure(ctx, healthKey(carrierID, tier), ClassifyError(evt.Content))
			}
		}
		out <- evt
	}
}
