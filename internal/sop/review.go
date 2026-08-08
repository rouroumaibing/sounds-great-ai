package sop

import (
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
	ReviewerID string    `json:"reviewer_id"`
	ReviewerBreed string `json:"reviewer_breed"`
	ReviewSHA  string    `json:"review_sha"`
	Timestamp  time.Time `json:"timestamp"`
	Status     string    `json:"status"`
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

// ReviewCycle manages the review request/receive cycle.
type ReviewCycle struct {
	requests   map[string]*ReviewRequest
	provenance []ReviewProvenance
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

// ReceiveReview records a review result and its provenance.
func (rc *ReviewCycle) ReceiveReview(provenance ReviewProvenance) {
	if provenance.Timestamp.IsZero() {
		provenance.Timestamp = time.Now()
	}
	rc.provenance = append(rc.provenance, provenance)
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

// IsCrossBreedReview checks if the review for a SHA was from a different breed.
func (rc *ReviewCycle) IsCrossBreedReview(sha, authorBreed string) bool {
	for _, p := range rc.provenance {
		if p.ReviewSHA == sha && p.ReviewerBreed != authorBreed {
			return true
		}
	}
	return false
}

// ErrNoReviewerAvailable is returned when no reviewer can be found.
var ErrNoReviewerAvailable = &reviewerError{"no reviewer available"}

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
