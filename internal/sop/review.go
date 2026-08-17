package sop

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// ReviewPolicy configures how a reviewer is selected.
type ReviewPolicy struct {
	RequireDifferentBreed bool
	RequireDifferentCLI  bool
	PreferredRoles       []string
	ExcludeUnavailable   bool
}

// ReviewResult holds the outcome of a review.
type ReviewResult struct {
	Status   string
	Comments string
	Reviewer string
}

// ReviewProvenance tracks who reviewed what and when.
type ReviewProvenance struct {
	ReviewerID      string    `json:"reviewer_id"`
	ReviewerBreed   string    `json:"reviewer_breed"`
	ReviewSHA       string    `json:"review_sha"`
	Timestamp       time.Time `json:"timestamp"`
	Status          string    `json:"status"`
	ReviewerDogID   string    `json:"reviewer_dog_id,omitempty"`
	ReviewerThreadID string   `json:"reviewer_thread_id,omitempty"`
	AuthorDogID     string    `json:"author_dog_id,omitempty"`
}

// ReviewRequest is a request for review from one breed to another.
type ReviewRequest struct {
	FromBreed   string    `json:"from_breed"`
	ToBreed     string    `json:"to_breed"`
	ArtifactSHA string    `json:"artifact_sha"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
}

// BreedInfo provides breed metadata for reviewer selection.
type BreedInfo struct {
	ID     string
	CLI    string
	Roles  []string
	CanReview []string
	CannotReviewSelf bool
	CrossBreedPreferred bool
	Available bool
}

// SelectReviewer selects a reviewer from candidates based on policy.
// Returns the selected reviewer ID, or empty string if none found.
func SelectReviewer(authorBreed string, candidates []string, policy ReviewPolicy) string {
	for _, c := range candidates {
		if policy.RequireDifferentBreed && c == authorBreed {
			continue
		}
		return c
	}
	return ""
}

// SelectReviewerFromBreeds selects a reviewer from BreedInfo list using
// the author's breed review policy. Enforces cross-breed and cross-CLI rules.
func SelectReviewerFromBreeds(author BreedInfo, candidates []BreedInfo, policy ReviewPolicy) (*BreedInfo, error) {
	var filtered []BreedInfo
	for _, c := range candidates {
		if !c.Available && policy.ExcludeUnavailable {
			continue
		}
		if policy.RequireDifferentBreed && c.ID == author.ID {
			continue
		}
		if author.CannotReviewSelf && c.ID == author.ID {
			continue
		}
		if policy.RequireDifferentCLI && c.CLI == author.CLI {
			continue
		}
		// If can_review is specified, candidate must be in the list
		if len(author.CanReview) > 0 && !containsID(author.CanReview, c.ID) {
			continue
		}
		filtered = append(filtered, c)
	}

	// Prefer candidates with preferred roles
	if len(policy.PreferredRoles) > 0 {
		for _, c := range filtered {
			for _, role := range c.Roles {
				if containsStr(role, policy.PreferredRoles[0]) || hasRole(c.Roles, policy.PreferredRoles) {
					_ = c
					return &filtered[0], nil
				}
			}
		}
	}

	// Prefer cross-breed if preferred
	if author.CrossBreedPreferred {
		for _, c := range filtered {
			if c.ID != author.ID {
				return &c, nil
			}
		}
	}

	if len(filtered) > 0 {
		return &filtered[0], nil
	}
	return nil, ErrNoReviewerAvailable
}

// ReviewPanel is the three-role cross-model review assignment (clowder F253):
//   - Layer 1 Hygiene: automated fixes, no named identity required.
//   - Layer 2 Reviewer: a named cross-breed / cross-CLI reviewer.
//   - Layer 3 FinalApprover: a second named identity, independent of both the
//     author and the reviewer.
type ReviewPanel struct {
	Reviewer      *BreedInfo
	FinalApprover *BreedInfo
}

// SelectReviewPanel picks a distinct reviewer and an independent final approver
// from the candidate breeds under policy. The reviewer must differ from the
// author; the final approver must differ from both the author and the reviewer.
// Returns ErrNoReviewerAvailable if no valid reviewer can be formed, or
// ErrNoFinalApproverAvailable if a reviewer exists but no independent approver.
func SelectReviewPanel(author BreedInfo, candidates []BreedInfo, policy ReviewPolicy) (*ReviewPanel, error) {
	reviewer, err := SelectReviewerFromBreeds(author, candidates, policy)
	if err != nil {
		return nil, err
	}
	// Exclude the chosen reviewer when selecting the final approver so the
	// two Layer-2/3 identities are never the same dog.
	var remain []BreedInfo
	for _, c := range candidates {
		if c.ID == reviewer.ID {
			continue
		}
		remain = append(remain, c)
	}
	approverPolicy := policy
	approverPolicy.RequireDifferentBreed = true
	finalApprover, err := SelectReviewerFromBreeds(author, remain, approverPolicy)
	if err != nil {
		return nil, ErrNoFinalApproverAvailable
	}
	return &ReviewPanel{Reviewer: reviewer, FinalApprover: finalApprover}, nil
}

// ReviewCycle manages the review request/receive cycle. Identity is tracked by
// dog_id (the canonical agent identity, resolved from the executing breed
// variant) so that the cross-model invariant is independent of breed labels and
// follows the model that actually performed the work.
type ReviewCycle struct {
	requests   map[string]*ReviewRequest
	provenance []ReviewProvenance
	// AuthorDogID is the identity that authored the work under review.
	AuthorDogID string
	// AssignedReviewerDogID is the identity designated to review. Once set, only
	// this identity may record the verdict (write-back fail-closed).
	AssignedReviewerDogID string
	// AssignedReviewerThreadID is the thread that issued the review request; the
	// verdict must return to this direct review carrier thread.
	AssignedReviewerThreadID string
}

// NewReviewCycle creates a new ReviewCycle.
func NewReviewCycle() *ReviewCycle {
	return &ReviewCycle{
		requests: make(map[string]*ReviewRequest),
	}
}

// RequestReview creates a review request.
func (rc *ReviewCycle) RequestReview(req ReviewRequest) {
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	rc.requests[req.ToBreed+":"+req.ArtifactSHA] = &req
}

// ReceiveReview records a review result and its provenance. It is the raw
// append used by the aggregate apply path and tests; runtime write-back must go
// through RecordReview, which fails closed on identity.
func (rc *ReviewCycle) ReceiveReview(provenance ReviewProvenance) {
	if provenance.Timestamp.IsZero() {
		provenance.Timestamp = time.Now()
	}
	rc.provenance = append(rc.provenance, provenance)
}

// AssignReview binds a review request to a designated reviewer identity. Only
// the assigned reviewer dog (AssignedReviewerDogID) may record the verdict via
// RecordReview, and the author (AuthorDogID) is never permitted to self-review.
func (rc *ReviewCycle) AssignReview(authorDogID, reviewerDogID, threadID string) {
	rc.AuthorDogID = authorDogID
	rc.AssignedReviewerDogID = reviewerDogID
	rc.AssignedReviewerThreadID = threadID
}

// RecordReview records a review verdict. It fails closed: the writer must carry
// a distinct identity from the author, and — once a reviewer has been assigned —
// must be the assigned reviewer. This is the write-back guard: a verdict whose
// principal is the author, or any dog other than the designated reviewer, is
// rejected rather than silently recorded.
func (rc *ReviewCycle) RecordReview(provenance ReviewProvenance) error {
	if provenance.ReviewerDogID == "" {
		return ErrReviewNoIdentity
	}
	if rc.AuthorDogID != "" && provenance.ReviewerDogID == rc.AuthorDogID {
		return ErrSelfReview
	}
	if rc.AssignedReviewerDogID != "" && provenance.ReviewerDogID != rc.AssignedReviewerDogID {
		return ErrWrongPrincipal
	}
	if rc.AssignedReviewerThreadID != "" && provenance.ReviewerThreadID != rc.AssignedReviewerThreadID {
		return ErrWrongCarrier
	}
	if provenance.Timestamp.IsZero() {
		provenance.Timestamp = time.Now()
	}
	rc.provenance = append(rc.provenance, provenance)
	return nil
}

// Provenance returns all review provenance records.
func (rc *ReviewCycle) Provenance() []ReviewProvenance {
	return rc.provenance
}

// HasReviewForSHA checks if a review exists for the given SHA.
func (rc *ReviewCycle) HasReviewForSHA(sha string) bool {
	for _, p := range rc.provenance {
		if p.ReviewSHA == sha {
			return true
		}
	}
	return false
}

// IsCrossBreedReview checks if the review for a SHA was from a different dog
// identity than the author.
func (rc *ReviewCycle) IsCrossBreedReview(sha, authorDogID string) bool {
	for _, p := range rc.provenance {
		if p.ReviewSHA == sha && p.ReviewerDogID != "" && p.ReviewerDogID != authorDogID {
			return true
		}
	}
	return false
}

// IsSelfReview reports whether a recorded review for sha was performed by the
// author's own dog identity. This is the negative of a genuine cross-breed
// review and is used to fail closed when provenance shows the author reviewed
// themselves.
func (rc *ReviewCycle) IsSelfReview(sha, authorDogID string) bool {
	for _, p := range rc.provenance {
		if p.ReviewSHA == sha && p.ReviewerDogID != "" && p.ReviewerDogID == authorDogID {
			return true
		}
	}
	return false
}

// ReviewerDelta quantifies the added value of a cross-model review. Findings
// tagged [delta:new] (or [FC:new]) are discoveries the prior review layer
// missed; [delta:covered] (or [FC:covered]) were already found; [delta:N/A]
// (or [FC:N/A]) excludes non-code findings. The ratio is new/(new+covered).
type ReviewerDelta struct {
	Covered int
	New     int
	NA      int
	Ratio   float64
}

// reviewerDeltaTag matches [delta:...] or [FC:...] annotations, case-insensitively.
var reviewerDeltaTag = regexp.MustCompile(`\[(?i)(?:delta|fc):(covered|new|n/?a)\]`)

// ComputeReviewerDelta parses reviewer delta annotations from review comments
// and returns the coverage vs. new-discovery counts plus the new-discovery
// ratio (New / (New + Covered)).
func ComputeReviewerDelta(comments string) ReviewerDelta {
	var d ReviewerDelta
	for _, m := range reviewerDeltaTag.FindAllStringSubmatch(comments, -1) {
		switch strings.ToLower(m[1]) {
		case "covered":
			d.Covered++
		case "new":
			d.New++
		case "n/a":
			d.NA++
		}
	}
	total := d.New + d.Covered
	if total > 0 {
		d.Ratio = float64(d.New) / float64(total)
	}
	return d
}

// ErrNoReviewerAvailable is returned when no reviewer can be found.
var ErrNoReviewerAvailable = &reviewerError{"no reviewer available"}

// ErrNoFinalApproverAvailable is returned when a reviewer identity exists but no
// independent final-approver identity can be formed for the three-role panel.
var ErrNoFinalApproverAvailable = &reviewerError{"no independent final approver available"}

// ErrReviewNoIdentity is returned when a review record carries no reviewer
// identity, so independence cannot be established.
var ErrReviewNoIdentity = errors.New("review record rejected: missing reviewer identity")

// ErrSelfReview is returned when the review writer is the author of the work.
var ErrSelfReview = errors.New("review record rejected: reviewer is the author (self-review)")

// ErrWrongPrincipal is returned when a non-assigned dog attempts to record the
// review verdict.
var ErrWrongPrincipal = errors.New("review record rejected: writer is not the assigned reviewer")

// ErrWrongCarrier is returned when the review verdict is written back to a
// thread other than the one that issued the review request.
var ErrWrongCarrier = errors.New("review record rejected: verdict returned to the wrong review thread")

type reviewerError struct{ msg string }

func (e *reviewerError) Error() string { return e.msg }

func containsID(list []string, id string) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}

func hasRole(roles, preferred []string) bool {
	for _, r := range roles {
		for _, p := range preferred {
			if r == p {
				return true
			}
		}
	}
	return false
}
