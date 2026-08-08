package cue

import (
	"strings"
	"time"
)

// OpportunityType is a typed predicate identifier from the closed catalog.
// Each predicate is a typed check, NOT an LLM judgment (VISION §3).
type OpportunityType string

const (
	OpSubjectSeen             OpportunityType = "subject_seen"
	OpDeliveryDecision        OpportunityType = "delivery_decision"
	OpJudgmentSurfaceEntered  OpportunityType = "judgment_surface_entered"
)

// allOpportunityTypes is the closed set of known opportunity types.
var allOpportunityTypes = map[OpportunityType]bool{
	OpSubjectSeen:            true,
	OpDeliveryDecision:       true,
	OpJudgmentSurfaceEntered: true,
}

// RecallOpportunity is a detected opportunity to recall memory.
type RecallOpportunity struct {
	Type      OpportunityType `json:"type"`
	Subject   string          `json:"subject"`   // what the opportunity is about
	Lane      string          `json:"lane"`      // suggested lane
	Reason    string          `json:"reason"`    // why this opportunity was detected
	Timestamp int64           `json:"timestamp"`
}

// CatalogInput is the typed input for opportunity detection.
// All fields are structured data extracted from the session — no raw text parsing.
type CatalogInput struct {
	// SubjectMentions: entities/subjects mentioned in the current turn
	SubjectMentions []string
	// DeliveryDecisionPending: true if a delivery/commit decision is being made
	DeliveryDecisionPending bool
	// JudgmentSurfaceEntered: true if the agent entered a judgment surface (review/eval)
	JudgmentSurfaceEntered bool
	// KnownSubjects: subjects already seen in prior turns (for subject_seen)
	KnownSubjects []string
}

// RecallOpportunityCatalog is a closed typed predicate catalog.
// It detects recall opportunities using typed checks, not LLM judgments.
// Unknown events return zero cues (fail-closed).
type RecallOpportunityCatalog struct{}

// NewCatalog creates a new RecallOpportunityCatalog.
func NewCatalog() *RecallOpportunityCatalog {
	return &RecallOpportunityCatalog{}
}

// Detect evaluates the typed input and returns recall opportunities.
// This is deterministic pattern matching — no LLM is called.
func (c *RecallOpportunityCatalog) Detect(input CatalogInput) []RecallOpportunity {
	var ops []RecallOpportunity
	now := time.Now().UnixMilli()

	// subject_seen: a subject mentioned now was seen before
	for _, subject := range input.SubjectMentions {
		for _, known := range input.KnownSubjects {
			if strings.EqualFold(subject, known) {
				ops = append(ops, RecallOpportunity{
					Type:      OpSubjectSeen,
					Subject:   subject,
					Lane:      "entity",
					Reason:    "subject previously seen in session",
					Timestamp: now,
				})
				break
			}
		}
	}

	// delivery_decision: a delivery/commit decision is pending
	if input.DeliveryDecisionPending {
		ops = append(ops, RecallOpportunity{
			Type:      OpDeliveryDecision,
			Subject:   "delivery",
			Lane:      "decision",
			Reason:    "delivery decision pending, recall prior decisions",
			Timestamp: now,
		})
	}

	// judgment_surface_entered: agent entered a review/eval surface
	if input.JudgmentSurfaceEntered {
		ops = append(ops, RecallOpportunity{
			Type:      OpJudgmentSurfaceEntered,
			Subject:   "judgment",
			Lane:      "lesson",
			Reason:    "judgment surface entered, recall lessons",
			Timestamp: now,
		})
	}

	return ops
}

// IsKnownType checks if an opportunity type is in the closed catalog.
func IsKnownType(t OpportunityType) bool {
	return allOpportunityTypes[t]
}

// AllOpportunityTypes returns the closed set of known opportunity types.
func AllOpportunityTypes() []OpportunityType {
	return []OpportunityType{OpSubjectSeen, OpDeliveryDecision, OpJudgmentSurfaceEntered}
}
