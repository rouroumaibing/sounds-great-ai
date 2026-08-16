// F276 People & Relationship Memory — frontend types (homologous).
// Mirrors the backend settings.PeopleMemoryStore contract.

export type ClaimKind = 'reported_fact' | 'user_assessment' | 'agent_inference' | 'redacted';
export type ClaimStatus = 'current' | 'superseded' | 'retired' | 'redacted';
export type RelStatus = 'current' | 'former' | 'unknown';
export type EventStatus = 'active' | 'redacted';
export type PersonStatus = 'active' | 'retired';
export type CandidateState =
  | 'pending_approval'
  | 'not_now'
  | 'partially_materialized'
  | 'materialized'
  | 'rejected'
  | 'withdrawn';

export interface SourceRef {
  source_kind?: string;
  thread_id?: string;
  message_id?: string;
  excerpt?: string;
  ref?: string;
}

export interface TemporalValue {
  kind: 'exact' | 'approximate' | 'conflict';
  value?: string;
  raw?: string;
  qualifier?: string;
  earliest?: string;
  latest?: string;
  alternatives?: { label: string; value: string }[];
}

export interface PersonClaimPayload {
  kind: ClaimKind;
  predicate?: string;
  value?: unknown;
  statement?: string;
  stance?: 'endorsed' | 'rejected' | 'uncertain';
}

export interface PersonClaimVersion {
  claim_id: string;
  person_id: string;
  payload: PersonClaimPayload;
  status: ClaimStatus;
  valid_from?: number;
  valid_to?: number;
  recorded_at: number;
  source_refs: SourceRef[];
  supersedes_claim_id?: string;
  decision_ref?: string;
}

export interface RelationshipTransition {
  status: RelStatus;
  recorded_at: number;
  source_refs: SourceRef[];
}

export interface PersonRelationship {
  relationship_id: string;
  person_id: string;
  status: RelStatus;
  created_at: number;
  source_refs: SourceRef[];
  transitions: RelationshipTransition[];
}

export interface InteractionEvent {
  event_id: string;
  relationship_id: string;
  occurred_at?: TemporalValue;
  duration?: TemporalValue;
  event_kind: string;
  headline: string;
  importance_or_topic?: string;
  uncertainty_notes?: string[];
  source_refs: SourceRef[];
  amends_event_id?: string;
  status: EventStatus;
  recorded_at: number;
}

export interface WorkspaceEntityLink {
  entity_ref: string;
  state: 'linked' | 'stale' | 'deleted';
  checked_at: number;
  superseded_by_entity_ref?: string;
}

export interface PersonIdentity {
  person_id: string;
  display_name: string;
  private_aliases: string[];
  status: PersonStatus;
  created_at: number;
  source_refs: SourceRef[];
  workspace_entity_link?: WorkspaceEntityLink;
}

export interface CandidateClaimDraft {
  draft_id: string;
  payload: PersonClaimPayload;
  normalized_draft: string;
  source_role: 'owner_explicit' | 'quoted_third_party';
  evidence_excerpt: string;
  decision: 'pending' | 'approved' | 'rejected';
}

export interface CandidateRelationshipDraft {
  draft_id: string;
  status: RelStatus;
  decision: 'pending' | 'approved' | 'rejected';
}

export interface CandidateInteractionDraft {
  draft_id: string;
  occurred_at?: TemporalValue;
  duration?: TemporalValue;
  event_kind: string;
  headline: string;
  importance_or_topic?: string;
  uncertainty_notes?: string[];
  decision: 'pending' | 'approved' | 'rejected';
}

export interface PersonIdentityDraft {
  display_name: string;
  private_aliases: string[];
  workspace_entity_link?: WorkspaceEntityLink;
}

export interface CaptureCandidate {
  candidate_id: string;
  requester_cat: string;
  source_message_ref: SourceRef;
  person_draft?: PersonIdentityDraft;
  target_person_id?: string;
  claim_drafts: CandidateClaimDraft[];
  relationship_draft?: CandidateRelationshipDraft;
  interaction_draft?: CandidateInteractionDraft;
  remaining_draft_ids: string[];
  state: CandidateState;
  presented_at: number;
  not_now_at?: number;
  deferred_receipt_id?: string;
  replaces_proposal_id?: string;
  replaced_by_proposal_id?: string;
  created_at: number;
  decision_refs?: string[];
}

export interface RelationshipCardFact {
  claim_id: string;
  text: string;
  kind: ClaimKind;
  provenance_refs: SourceRef[];
}

export interface RelationshipCardInteraction {
  event_id: string;
  occurred_at?: TemporalValue;
  headline: string;
}

export interface RelationshipCard {
  person_id: string;
  relationship_id: string;
  display_name: string;
  facts: RelationshipCardFact[];
  relationship_line?: string;
  latest_interaction?: RelationshipCardInteraction;
  uncertainty: string[];
  provenance_refs: SourceRef[];
  storable: boolean;
  indexable: boolean;
}

export interface PersonMemoryDecisionReceipt {
  decision_id: string;
  candidate_id: string;
  person_id: string;
  selected_draft_ids: string[];
  materialized_claim_ids: string[];
  materialized_event_ids: string[];
  restored_claim_ids: string[];
  created_relationship: boolean;
  relationship_status?: string;
  remaining_draft_ids: string[];
  decided_at: number;
}

export interface DeferredPersonMemoryReceipt {
  receipt_id: string;
  owner_user_id: string;
  requester_cat: string;
  subject: string;
  person_id?: string;
  source_coords: SourceRef[];
  created_at: number;
  claimed_at?: number;
  claim_id?: string;
  withdrawn?: boolean;
}

export interface PersonMemoryDeletionReceipt {
  request_id: string;
  person_id?: string;
  proposal_id?: string;
  verdict: 'purged' | 'already_absent';
  counts: Record<string, number>;
}

export interface PersonSummary {
  person_id: string;
  display_name: string;
  aliases: string[];
  status: PersonStatus;
  current_claims: number;
  events: number;
}
