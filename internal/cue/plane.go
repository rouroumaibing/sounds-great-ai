package cue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CuePlane orchestrates the cue pipeline: catalog → resolver → envelope → injection.
// It enforces budget, dedup, and expiry.
type CuePlane struct {
	catalog    *RecallOpportunityCatalog
	resolvers  *LaneResolverRegistry
	builder    *EnvelopeBuilder
	ledger     *ConsumptionLedger
	mu         sync.Mutex
}

// NewCuePlane creates a new CuePlane with all components.
func NewCuePlane(
	catalog *RecallOpportunityCatalog,
	resolvers *LaneResolverRegistry,
	builder *EnvelopeBuilder,
	ledger *ConsumptionLedger,
) *CuePlane {
	return &CuePlane{
		catalog:   catalog,
		resolvers: resolvers,
		builder:   builder,
		ledger:    ledger,
	}
}

// InjectResult is the result of a cue injection.
type InjectResult struct {
	Envelopes []*CueEnvelope `json:"envelopes"`
	Injected  int            `json:"injected"`
	Skipped   int            `json:"skipped"`
	Prompt    string         `json:"prompt"` // the assembled prompt fragment
}

// Inject detects recall opportunities, resolves content, builds envelopes,
// and assembles a prompt fragment for injection.
// Budget/dedupe/expiry are enforced. Source invalidation causes fail-closed.
func (cp *CuePlane) Inject(ctx context.Context, input CatalogInput, sessionID string) (*InjectResult, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	result := &InjectResult{}

	// Step 1: Detect recall opportunities (typed predicates, no LLM)
	ops := cp.catalog.Detect(input)

	// Step 2: For each opportunity, resolve content from the lane
	for _, op := range ops {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		resolved, err := cp.resolvers.Resolve(ctx, op.Lane, op.Subject, op.Reason, BudgetForLane(op.Lane))
		if err != nil {
			// fail-closed: skip on error
			result.Skipped++
			continue
		}

		// Step 3: Build envelopes from resolved content
		for _, rc := range resolved {
			// Source invalidation check: fail-closed
			if cp.ledger.IsSourceInvalidated(rc.SourceID) {
				result.Skipped++
				continue
			}

			envelopeID := uuid.NewString()
			env := cp.builder.Build(
				envelopeID,
				rc.Lane,
				op.Reason,
				rc.Content,
				rc.SourceID,
				fmt.Sprintf("drill:%s:%s", rc.Lane, rc.SourceID),
				ScopeSession,
				time.Now().Add(30*time.Minute).UnixMilli(), // 30 min expiry
				fmt.Sprintf("source:%s invalidated", rc.SourceID),
			)
			if env == nil {
				// dedup: already built
				result.Skipped++
				continue
			}

			// Check expiry
			if env.IsExpired() {
				result.Skipped++
				continue
			}

			result.Envelopes = append(result.Envelopes, env)

			// Record consumption episode (presented)
			cp.ledger.RecordPresented(env.ID, env.Lane, sessionID)
		}
	}

	// Step 4: Assemble prompt fragment
	result.Prompt = cp.AssemblePrompt(result.Envelopes)
	result.Injected = len(result.Envelopes)

	return result, nil
}

// AssemblePrompt assembles cue envelopes into a prompt fragment.
func (cp *CuePlane) AssemblePrompt(envelopes []*CueEnvelope) string {
	if len(envelopes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<cue-plane>\n")
	for _, env := range envelopes {
		sb.WriteString(fmt.Sprintf("  <cue lane=\"%s\" why=\"%s\">\n", env.Lane, env.WhyNow))
		sb.WriteString(fmt.Sprintf("    %s\n", env.Summary))
		sb.WriteString(fmt.Sprintf("    drill: %s\n", env.DrillHandle))
		sb.WriteString("  </cue>\n")
	}
	sb.WriteString("</cue-plane>")
	return sb.String()
}

// RecordAgentAction records how the agent consumed a cue.
func (cp *CuePlane) RecordAgentAction(envelopeID, lane, sessionID string, action ConsumptionEpisodeAction) {
	switch action {
	case EpisodeDrilled:
		cp.ledger.RecordDrilled(envelopeID, lane, sessionID)
	case EpisodeApplied:
		cp.ledger.RecordApplied(envelopeID, lane, sessionID)
	case EpisodeDismissed:
		cp.ledger.RecordDismissed(envelopeID, lane, sessionID)
	}
}

// InvalidateSource invalidates a source, causing all cues from it to fail-closed.
func (cp *CuePlane) InvalidateSource(sourceID, reason string) {
	cp.ledger.InvalidateSource(sourceID, reason)
}

// Reset clears the dedup set for a new invocation.
func (cp *CuePlane) Reset() {
	cp.builder.Reset()
}

// Ledger returns the consumption ledger for inspection.
func (cp *CuePlane) Ledger() *ConsumptionLedger {
	return cp.ledger
}
