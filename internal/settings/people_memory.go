package settings

import (
	"time"

	"github.com/google/uuid"
)

// This file defines the Persistent-Identity F276 "People & Relationship Memory"
// contract types (homologous) for Sounds Great AI. SG keeps these as six logical
// objects, partitioned by operatorID for multi-operator support (file-backed by
// default, optional Redis-backed — see people_memory_redis.go), written
// atomically under ConfigRoot.
//
// Design discipline (faithful to F276 / KD-1..KD-12):
//   - third-party truth is owner-private and never silently written: a capture
//     candidate must be explicitly approved before it materializes.
//   - reported_fact / user_assessment are materializable; agent_inference has
//     NO approval path (kept ephemeral, never persisted as truth).
//   - every materialized claim/event/relationship carries typed provenance
//     (SourceRefs) so corrections and forget are traceable.
//   - claims are versioned: a new current claim supersedes the old one (status
//     goes current -> superseded), never overwritten.
//   - the RelationshipCard is a derived, non-storable projection (storable:false
//     / indexable:false) — recall reads it; nothing writes it.
//   - a bounded dual path: clear, in-turn cues are proposed immediately;
//     worth-remembering-but-not-now cues become content-free deferred receipts
//     that a daily clerk can later claim into a normal proposal.
//
// No LLM reasoning runs inside the platform (docs/decisions/irreversible-decisions.md §4.1): this package only
// stores and projects what an operator or CLI dog submits.

// F276 claim kinds. agent_inference is accepted only as a draft signal and is
// never materialized as canonical truth (AC-A3).
const (
	ClaimKindReportedFact   = "reported_fact"
	ClaimKindUserAssessment = "user_assessment"
	ClaimKindAgentInference = "agent_inference"
	ClaimKindRedacted       = "redacted"
)

// Claim/relationship/event status enums.
const (
	ClaimStatusCurrent    = "current"
	ClaimStatusSuperseded = "superseded"
	ClaimStatusRetired    = "retired"
	ClaimStatusRedacted   = "redacted"

	RelStatusCurrent = "current"
	RelStatusFormer  = "former"
	RelStatusUnknown = "unknown"

	EventStatusActive    = "active"
	EventStatusRedacted  = "redacted"
	PersonStatusActive   = "active"
	PersonStatusRetired  = "retired"
	WorkspaceLinkLinked  = "linked"
	WorkspaceLinkStale   = "stale"
	WorkspaceLinkDeleted = "deleted"
)

// CaptureCandidate lifecycle states (PERSON_MEMORY_CANDIDATE_STATES).
const (
	CandPendingApproval       = "pending_approval"
	CandNotNow                = "not_now"
	CandPartiallyMaterialized = "partially_materialized"
	CandMaterialized          = "materialized"
	CandRejected              = "rejected"
	CandWithdrawn             = "withdrawn"
)

// SourceRef is the typed provenance of a memory item. It records where the
// knowledge came from (who said it, in which thread/message) and a bounded
// verbatim excerpt the operator can drill into — but never the full private
// body (KD-7 privacy discipline). Source coordinates are server-derived where
// possible; the operator/CLI supplies only the bounded subject.
type SourceRef struct {
	SourceKind string `json:"source_kind,omitempty"` // message_text | operator | manual | transcript
	ThreadID   string `json:"thread_id,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	Excerpt    string `json:"excerpt,omitempty"` // bounded verbatim excerpt (owner-visible only)
	Ref        string `json:"ref,omitempty"`     // opaque digest/ref
}

// TemporalValue mirrors the shared TemporalValue schema: an occurred-at
// or duration value that may be exact, approximate (with qualifier), or a
// conflict (multiple incompatible alternatives). Never exposed as `any`.
type TemporalValue struct {
	Kind         string                `json:"kind"` // exact | approximate | conflict
	Value        string                `json:"value,omitempty"`
	Raw          string                `json:"raw,omitempty"`
	Qualifier    string                `json:"qualifier,omitempty"` // about | before | after | range | unknown
	Earliest     string                `json:"earliest,omitempty"`
	Latest       string                `json:"latest,omitempty"`
	Alternatives []TemporalAlternative `json:"alternatives,omitempty"`
}

// TemporalAlternative is one branch of a conflict value.
type TemporalAlternative struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// PersonClaimPayload is the discriminated union of a claim's content.
type PersonClaimPayload struct {
	Kind      string `json:"kind"` // reported_fact | user_assessment | redacted
	Predicate string `json:"predicate,omitempty"`
	Value     any    `json:"value,omitempty"`
	Statement string `json:"statement,omitempty"`
	Stance    string `json:"stance,omitempty"` // endorsed | rejected | uncertain
}

// PersonClaimVersion is a single versioned claim about a person (object 2).
// Only reported_fact / user_assessment become materialized truth; agent_inference
// never reaches this struct as canonical (it is rejected at the API boundary).
type PersonClaimVersion struct {
	ClaimID           string             `json:"claim_id"`
	PersonID          string             `json:"person_id"`
	Payload           PersonClaimPayload `json:"payload"`
	Status            string             `json:"status"` // current | superseded | retired | redacted
	ValidFrom         int64              `json:"valid_from,omitempty"`
	ValidTo           int64              `json:"valid_to,omitempty"`
	RecordedAt        int64              `json:"recorded_at"`
	SourceRefs        []SourceRef        `json:"source_refs"`
	SupersedesClaimID string             `json:"supersedes_claim_id,omitempty"`
	DecisionRef       string             `json:"decision_ref,omitempty"` // links to the decision receipt
}

// RelationshipTransition is an append-only status change on a relationship.
type RelationshipTransition struct {
	Status     string      `json:"status"` // current | former | unknown
	RecordedAt int64       `json:"recorded_at"`
	SourceRefs []SourceRef `json:"source_refs"`
}

// PersonRelationship is the You↔person relationship identity + lifecycle (object 3).
type PersonRelationship struct {
	RelationshipID string                  `json:"relationship_id"`
	PersonID       string                  `json:"person_id"`
	Status         string                  `json:"status"` // current | former | unknown
	CreatedAt      int64                   `json:"created_at"`
	SourceRefs     []SourceRef             `json:"source_refs"`
	Transitions    []RelationshipTransition `json:"transitions"`
}

// InteractionEvent is an append-only record of a meaningful interaction (object 4).
type InteractionEvent struct {
	EventID            string         `json:"event_id"`
	RelationshipID     string         `json:"relationship_id"`
	OccurredAt         *TemporalValue `json:"occurred_at,omitempty"`
	Duration           *TemporalValue `json:"duration,omitempty"`
	EventKind          string         `json:"event_kind"` // conversation | meeting | message | milestone | other
	Headline           string         `json:"headline"`
	ImportanceOrTopic  string         `json:"importance_or_topic,omitempty"`
	UncertaintyNotes   []string       `json:"uncertainty_notes,omitempty"`
	SourceRefs         []SourceRef    `json:"source_refs"`
	AmendsEventID      string         `json:"amends_event_id,omitempty"`
	Status             string         `json:"status"` // active | redacted
	RecordedAt         int64          `json:"recorded_at"`
}

// WorkspaceEntityLink is the one-way, server-derived link from a private person
// extension to the single active workspace person Entity (KD-12). The private
// dossier never writes back to the workspace Entity.
type WorkspaceEntityLink struct {
	EntityRef             string `json:"entity_ref"`
	State                 string `json:"state"` // linked | stale | deleted
	CheckedAt             int64  `json:"checked_at"`
	SupersededByEntityRef string `json:"superseded_by_entity_ref,omitempty"`
}

// PersonIdentity is the stable, owner-private person identity root (object 1).
type PersonIdentity struct {
	PersonID           string                `json:"person_id"`
	DisplayName        string                `json:"display_name"`
	PrivateAliases     []string              `json:"private_aliases"`
	Status             string                `json:"status"` // active | retired
	CreatedAt          int64                 `json:"created_at"`
	SourceRefs         []SourceRef           `json:"source_refs"`
	WorkspaceEntityLink *WorkspaceEntityLink `json:"workspace_entity_link,omitempty"`
}

// CandidateClaimDraft is one exact-bind, approvable draft inside a candidate.
type CandidateClaimDraft struct {
	DraftID         string             `json:"draft_id"`
	Payload         PersonClaimPayload `json:"payload"`
	NormalizedDraft string             `json:"normalized_draft"`
	SourceRole      string             `json:"source_role"` // owner_explicit | quoted_third_party
	EvidenceExcerpt string             `json:"evidence_excerpt"`
	Decision        string             `json:"decision"` // pending | approved | rejected
}

// CandidateRelationshipDraft carries a relationship status proposal.
type CandidateRelationshipDraft struct {
	DraftID string `json:"draft_id"`
	Status  string `json:"status"` // current | former | unknown
	Decision string `json:"decision"`
}

// CandidateInteractionDraft carries an interaction event proposal.
type CandidateInteractionDraft struct {
	DraftID         string         `json:"draft_id"`
	OccurredAt      *TemporalValue `json:"occurred_at,omitempty"`
	Duration        *TemporalValue `json:"duration,omitempty"`
	EventKind       string         `json:"event_kind"`
	Headline        string         `json:"headline"`
	ImportanceOrTopic string       `json:"importance_or_topic,omitempty"`
	UncertaintyNotes  []string     `json:"uncertainty_notes,omitempty"`
	Decision        string         `json:"decision"`
}

// PersonIdentityDraft is the identity portion of a capture candidate.
type PersonIdentityDraft struct {
	DisplayName         string               `json:"display_name"`
	PrivateAliases      []string             `json:"private_aliases"`
	WorkspaceEntityLink *WorkspaceEntityLink `json:"workspace_entity_link,omitempty"`
}

// CaptureCandidate is the approval envelope (object 5): a presented, owner
// private proposal with per-draft authorization state. It is NOT canonical
// truth; only an explicit approval materializes it.
type CaptureCandidate struct {
	CandidateID          string                  `json:"candidate_id"`
	RequesterDog         string                  `json:"requester_dog"`
	SourceMessageRef     SourceRef               `json:"source_message_ref"`
	PersonDraft          *PersonIdentityDraft    `json:"person_draft,omitempty"`
	TargetPersonID       string                  `json:"target_person_id,omitempty"`
	ClaimDrafts          []CandidateClaimDraft   `json:"claim_drafts"`
	RelationshipDraft    *CandidateRelationshipDraft `json:"relationship_draft,omitempty"`
	InteractionDraft     *CandidateInteractionDraft  `json:"interaction_draft,omitempty"`
	RemainingDraftIDs    []string                `json:"remaining_draft_ids"`
	State                string                  `json:"state"` // candidate states
	PresentedAt          int64                   `json:"presented_at"`
	NotNowAt             int64                   `json:"not_now_at,omitempty"`
	DeferredReceiptID    string                  `json:"deferred_receipt_id,omitempty"`
	ReplacesProposalID   string                  `json:"replaces_proposal_id,omitempty"`
	ReplacedByProposalID string                  `json:"replaced_by_proposal_id,omitempty"`
	CreatedAt            int64                   `json:"created_at"`
	DecisionRefs         []string                `json:"decision_refs,omitempty"`
}

// DeferredPersonMemoryReceipt is the content-free, exact-source-bound receipt of
// the dual path (AC-A20): it persists only server-derived owner/breed/origin,
// bounded subject, exact source coordinates/digests — NEVER the message body,
// excerpt, transcript, or relationship fact. A daily clerk claims it back into a
// normal proposal.
type DeferredPersonMemoryReceipt struct {
	ReceiptID    string      `json:"receipt_id"`
	OwnerUserID  string      `json:"owner_user_id"`
	RequesterDog string      `json:"requester_dog"`
	Subject      string      `json:"subject"` // bounded subject only
	PersonID     string      `json:"person_id,omitempty"`
	SourceCoords []SourceRef `json:"source_coords"` // exact typed coordinates/digests
	CreatedAt    int64       `json:"created_at"`
	ClaimedAt    int64       `json:"claimed_at,omitempty"`
	ClaimID      string      `json:"claim_id,omitempty"` // candidate produced when claimed
	Withdrawn    bool        `json:"withdrawn,omitempty"`
}

// PersonMemoryDecisionReceipt is a content-free record of an approval/undo so it
// can be undone. It references the materialized items by id (no payload).
type PersonMemoryDecisionReceipt struct {
	DecisionID           string   `json:"decision_id"`
	CandidateID          string   `json:"candidate_id"`
	PersonID             string   `json:"person_id"`
	SelectedDraftIDs     []string `json:"selected_draft_ids"`
	MaterializedClaimIDs []string `json:"materialized_claim_ids"`
	MaterializedEventIDs []string `json:"materialized_event_ids"`
	RestoredClaimIDs     []string `json:"restored_claim_ids"` // superseded claims flipped back to current on undo
	CreatedRelationship  bool     `json:"created_relationship,omitempty"`
	RelationshipStatus   string   `json:"relationship_status,omitempty"`
	RemainingDraftIDs    []string `json:"remaining_draft_ids"`
	DecidedAt            int64    `json:"decided_at"`
}

// peopleMemoryDocument is the in-memory owner-private keyspace for ONE operator.
// Both the file store and the Redis store reuse this exact model; only the
// persistence substrate differs.
type peopleMemoryDocument struct {
	Version       int                                `json:"version"`
	People        map[string]*PersonIdentity         `json:"people"`
	Claims        map[string][]*PersonClaimVersion   `json:"claims"`
	Relationships map[string][]*PersonRelationship   `json:"relationships"`
	Events        map[string][]*InteractionEvent     `json:"events"`
	Candidates    map[string]*CaptureCandidate       `json:"candidates"`
	Receipts      map[string]*DeferredPersonMemoryReceipt `json:"receipts"`
	Decisions     map[string]*PersonMemoryDecisionReceipt `json:"decisions"`
}

func newPeopleMemoryDocument() *peopleMemoryDocument {
	return &peopleMemoryDocument{
		Version:       1,
		People:        map[string]*PersonIdentity{},
		Claims:        map[string][]*PersonClaimVersion{},
		Relationships: map[string][]*PersonRelationship{},
		Events:        map[string][]*InteractionEvent{},
		Candidates:    map[string]*CaptureCandidate{},
		Receipts:      map[string]*DeferredPersonMemoryReceipt{},
		Decisions:     map[string]*PersonMemoryDecisionReceipt{},
	}
}

// nowMs returns the current Unix millisecond timestamp.
func peopleNowMs() int64 { return time.Now().UnixMilli() }

// RedactTarget identifies a claim or event to redact (AC-B6).
type RedactTarget struct {
	Kind string `json:"kind"` // claim | event
	ID   string `json:"id"`
}

// ensure uuid stays imported (used by document methods in sibling files).
var _ = uuid.NewString
