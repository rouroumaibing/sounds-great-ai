package services

import (
	"os"
	"path/filepath"
	"sync"

	"sounds-great-ai/internal/a2a"
	sopPorts "sounds-great-ai/internal/domains/sop/ports"
	sop "sounds-great-ai/internal/sop"
)

// defaultSOPDefinitionPath is the bundled SOP definition. It is overridden via
// SetDefinitionPath (runtime) or the SG_SOP_DEFINITION env var.
var defaultSOPDefinitionPath = "packs/default/sop/development.yaml"

// SOPGuardianService adapts the flat *sop.SOPGuardian to the hexagonal
// IA2AGuardian port used by the runtime. It owns no new logic — it only
// translates between the port's types (EscalationAction/ReviewPolicy) and the
// concrete sop package types.
type SOPGuardianService struct {
	inner *sop.SOPGuardian

	defPath string
	def     *sop.SOPDefinition
	defOnce sync.Once

	// per-thread review assignment (lease) and cycle state, used to enforce
	// the write-back. A review lease is issued when a cross-dog handoff begins
	// a review; only the lease holder (the assigned reviewer) may return the
	// verdict, and it must return to the direct review thread.
	assignMu     sync.Mutex
	assignments  map[string]reviewAssignment
	authorAssign map[string]reviewAssignment
	cycles       map[string]*sop.ReviewCycle
	assignGen    uint64

	// reviewCompleteHook, when set, fires best-effort (never blocking the
	// handoff verdict) after a review provenance is recorded. FT-DS-001 uses
	// it to create distillation opportunities for the reviewed author.
	reviewCompleteHook func(sop.ReviewProvenance)
}

// reviewAssignment is the review lease. AuthorDogID is the predecessor (the
// work's author); ReviewerDogID is the lease holder (the assigned reviewer);
// ThreadID is the predecessor thread that issued the review request and is also
// the thread that holds the lease; Generation detects stale reuse.
type reviewAssignment struct {
	AuthorDogID    string
	ReviewerDogID  string
	ThreadID       string
	HolderThreadID string
	Generation     uint64
}

// NewSOPGuardianService wraps a concrete guardian in the port interface.
func NewSOPGuardianService(inner *sop.SOPGuardian) *SOPGuardianService {
	return &SOPGuardianService{
		inner:        inner,
		assignments:  make(map[string]reviewAssignment),
		authorAssign: make(map[string]reviewAssignment),
		cycles:       make(map[string]*sop.ReviewCycle),
	}
}

// SetReviewCompleteListener attaches a best-effort post-review hook
// (FT-DS-001 distillation checkpoint). Nil-safe.
func (s *SOPGuardianService) SetReviewCompleteListener(hook func(sop.ReviewProvenance)) {
	s.reviewCompleteHook = hook
}

// SetDefinitionPath overrides the SOP definition YAML used for declarative
// review-handoff enforcement.
func (s *SOPGuardianService) SetDefinitionPath(path string) {
	s.defPath = path
}

// loadDefinition resolves and caches the SOP definition. Resolution order:
// explicit path, then SG_SOP_DEFINITION env, then walking up from the working
// directory to find the bundled default. A missing file yields nil, in which
// case only the baseline invariant is enforced.
func (s *SOPGuardianService) loadDefinition() *sop.SOPDefinition {
	s.defOnce.Do(func() {
		path := resolveSOPDefinition(s.defPath)
		if path == "" {
			return
		}
		def, err := sop.LoadDefinition(path)
		if err != nil {
			return
		}
		s.def = def
	})
	return s.def
}

// resolveSOPDefinition finds the SOP definition YAML, tolerating launch cwd
// differences by walking up the directory tree.
func resolveSOPDefinition(explicit string) string {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := os.Getenv("SG_SOP_DEFINITION"); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates, defaultSOPDefinitionPath)
	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "packs/default/sop/development.yaml")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

// CheckA2ADepth delegates to the wrapped guardian, mapping its enum to the
// port's EscalationAction.
func (s *SOPGuardianService) CheckA2ADepth(thread *a2a.Thread) sopPorts.EscalationAction {
	switch s.inner.CheckA2ADepth(thread) {
	case sop.EscalateToCVO:
		return sopPorts.EscalateToCVO
	case sop.Block:
		return sopPorts.Block
	default:
		return sopPorts.Continue
	}
}

// SelectReviewer translates the port policy into the concrete sop policy and
// delegates.
func (s *SOPGuardianService) SelectReviewer(authorBreed string, candidates []string, policy sopPorts.ReviewPolicy) string {
	return sop.SelectReviewer(authorBreed, candidates, sop.ReviewPolicy{
		RequireDifferentBreed: policy.RequireDifferentBreed,
		RequireDifferentCLI:   policy.RequireDifferentCLI,
		PreferredRoles:        policy.PreferredRoles,
		ExcludeUnavailable:    policy.ExcludeUnavailable,
	})
}

// MaxA2ADepth reports the configured depth ceiling.
func (s *SOPGuardianService) MaxA2ADepth() int {
	return s.inner.MaxA2ADepth()
}

// EnforceReviewHandoff enforces the cross-model review invariant on an A2A
// handoff. The baseline rule is fail-closed: a dog cannot hand its own authored
// work to itself for review. When the SOP definition is available, the `review`
// stage blocker hard_rules (e.g. reviewer_not_author) are also evaluated. The
// first cross-dog handoff in a thread is treated as the review assignment; a
// later handoff that returns the work to the author is the write-back and is
// recorded via RecordReview (which fails closed on principal and carrier).
func (s *SOPGuardianService) EnforceReviewHandoff(in sopPorts.ReviewHandoffInput) sopPorts.ReviewHandoffVerdict {
	fromID := firstNonEmpty(in.AuthorDogID, in.AuthorBreed)
	toID := firstNonEmpty(in.ReviewerDogID, in.ReviewerBreed)

	// Baseline invariant: a dog may not hand its own authored work to itself
	// for review. This fails closed regardless of definition availability.
	if fromID != "" && fromID == toID {
		return sopPorts.ReviewHandoffVerdict{
			Blocked:  true,
			Messages: []string{"self-review rejected: reviewer dog_id must differ from author dog_id"},
		}
	}

	// Declarative blockers from the SOP definition (e.g. reviewer_not_author).
	if msgs := s.evalReviewBlockers(fromID, toID); len(msgs) > 0 {
		return sopPorts.ReviewHandoffVerdict{Blocked: true, Messages: msgs}
	}

	// Review lease issuance and write-back enforcement. A cross-dog handoff
	// issues a review lease; the write-back (the assigned reviewer returning
	// the verdict to the author) is gated by the lease terminal-route check,
	// which fails closed on any identity or carrier deviation.
	if in.SessionID != "" {
		s.assignMu.Lock()
		key := reviewKey(fromID, toID)
		asn := s.assignments[key]
		if asn == (reviewAssignment{}) && toID != "" {
			asn = s.authorAssign[toID]
		}
		if asn == (reviewAssignment{}) {
			// First cross-dog handoff: issue the review lease. Keyed by the
			// review pair so a later write-back arriving in a different thread
			// is detected as a carrier mismatch rather than a fresh assignment.
			if fromID != "" && toID != "" && fromID != toID {
				s.assignGen++
				asn = reviewAssignment{
					AuthorDogID:    fromID,
					ReviewerDogID:  toID,
					ThreadID:       in.SessionID,
					HolderThreadID: in.SessionID,
					Generation:     s.assignGen,
				}
				s.assignments[key] = asn
				s.authorAssign[fromID] = asn
			}
			s.assignMu.Unlock()
		} else {
			// A lease exists. A handoff that returns the work to the author is
			// the review write-back; gate it as a lease terminal route.
			targetsAuthor := asn.AuthorDogID == toID
			if targetsAuthor {
				allow, reason := s.preflightReviewTerminalRoute(in, asn)
				if !allow {
					s.assignMu.Unlock()
					return sopPorts.ReviewHandoffVerdict{
						Blocked:  true,
						Messages: []string{"review terminal route rejected: " + reason},
					}
				}
				cyc := s.cycles[key]
				if cyc == nil {
					cyc = sop.NewReviewCycle()
					cyc.AssignReview(asn.AuthorDogID, asn.ReviewerDogID, asn.ThreadID)
					s.cycles[key] = cyc
				}
				prov := sop.ReviewProvenance{
					ReviewerDogID:    asn.ReviewerDogID,
					AuthorDogID:      asn.AuthorDogID,
					ReviewerThreadID: in.SessionID,
					ReviewSHA:        in.SessionID,
				}
				err := cyc.RecordReview(prov)
				s.assignMu.Unlock()
				if err != nil {
					return sopPorts.ReviewHandoffVerdict{Blocked: true, Messages: []string{err.Error()}}
				}
				// FT-DS-001: best-effort distillation checkpoint — a review
				// just completed for the author; never block the verdict on it.
				if s.reviewCompleteHook != nil {
					func() {
						defer func() { _ = recover() }()
						s.reviewCompleteHook(prov)
					}()
				}
			} else {
				s.assignMu.Unlock()
			}
		}
	}
	return sopPorts.ReviewHandoffVerdict{}
}

// preflightReviewTerminalRoute validates that a review write-back returns to
// the direct review carrier thread and is performed by the lease holder. It is
// the terminal-route guard for the review lease: a verdict may only be
// persisted by the assigned reviewer into the thread that issued the review
// request. Any deviation fails closed with a named reason:
//   - predecessor_route_missing: no review thread was recorded
//   - generation_mismatch: the handoff generation does not match the lease
//   - reviewer_not_holder: the writer is not the assigned reviewer
//   - holder_thread_mismatch: the handoff thread is not the lease holder thread
//   - target_thread_mismatch: the verdict does not return to the review thread
func (s *SOPGuardianService) preflightReviewTerminalRoute(in sopPorts.ReviewHandoffInput, asn reviewAssignment) (bool, string) {
	if asn.ThreadID == "" {
		return false, "predecessor_route_missing"
	}
	if in.Generation != 0 && in.Generation != asn.Generation {
		return false, "generation_mismatch"
	}
	holderID := firstNonEmpty(in.AuthorDogID, in.AuthorBreed)
	if holderID != asn.ReviewerDogID {
		return false, "reviewer_not_holder"
	}
	if in.SessionID != asn.HolderThreadID {
		return false, "holder_thread_mismatch"
	}
	if in.SessionID != asn.ThreadID {
		return false, "target_thread_mismatch"
	}
	return true, ""
}

// evalReviewBlockers evaluates the `review` stage blocker hard_rules against
// the given author/reviewer identities and returns any violation messages.
func (s *SOPGuardianService) evalReviewBlockers(authorID, reviewerID string) []string {
	def := s.loadDefinition()
	if def == nil {
		return nil
	}
	stage := def.FindStage("review")
	if stage == nil {
		return nil
	}
	exec := sop.NewPredicateExecutor()
	ctx := sop.PredicateContext{Author: authorID, Reviewer: reviewerID}
	var msgs []string
	for _, rule := range stage.BlockerRules() {
		if rule.Predicate.Type == "" {
			continue
		}
		res := exec.Execute(rule.Predicate, ctx)
		if !res.Passed {
			msgs = append(msgs, rule.ID+": "+res.Message)
		}
	}
	return msgs
}

// firstNonEmpty returns the first non-empty string, used to resolve a dog_id
// with a breed fallback.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// reviewKey derives a stable, order-independent key for a review designation so
// the assignment and write-back can be correlated regardless of which side
// initiates the handoff.
func reviewKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}
