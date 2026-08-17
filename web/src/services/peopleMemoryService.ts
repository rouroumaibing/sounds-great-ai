// F276 People & Relationship Memory — API client (homologous).
import { apiGet, apiPost } from './http';
import type {
  PersonSummary,
  PersonIdentity,
  PersonClaimVersion,
  PersonRelationship,
  InteractionEvent,
  RelationshipCard,
  CaptureCandidate,
  DeferredPersonMemoryReceipt,
  PersonMemoryDecisionReceipt,
  PersonMemoryDeletionReceipt,
  PersonClaimPayload,
  CandidateInteractionDraft,
  SourceRef,
} from '../types/peopleMemory';

// --- Multi-operator scoping -------------------------------------------------
// The backend partitions people-memory by owner (operatorID) and resolves the
// scope from the X-Operator-Id header, falling back to the platform leader when
// the header is absent. We persist the operator the user selected so the whole
// panel stays scoped across reloads (KD-1 owner-partitioned memory).

const OPERATOR_KEY = 'sounds-great-ai:people-operator';

let activeOperator: string | null = localStorage.getItem(OPERATOR_KEY);

export function getActiveOperator(): string | null {
  return activeOperator;
}

export function setActiveOperator(op: string | null): void {
  const trimmed = op && op.trim() ? op.trim() : null;
  activeOperator = trimmed;
  if (trimmed) localStorage.setItem(OPERATOR_KEY, trimmed);
  else localStorage.removeItem(OPERATOR_KEY);
}

function operatorHeaders(): Record<string, string> {
  return activeOperator ? { 'X-Operator-Id': activeOperator } : {};
}

export interface PersonDetail {
  person: PersonIdentity;
  claims: PersonClaimVersion[];
  relationships: PersonRelationship[];
  events: InteractionEvent[];
  card: RelationshipCard | null;
  has_card: boolean;
}

export async function listOperators(): Promise<string[]> {
  // Operator-discovery is itself un-scoped (returns all operators).
  return apiGet<string[]>('/api/people-memory/operators');
}

export async function listPeople(): Promise<PersonSummary[]> {
  return apiGet<PersonSummary[]>('/api/people-memory', operatorHeaders());
}

export async function getPerson(personID: string): Promise<PersonDetail> {
  return apiGet<PersonDetail>(`/api/people-memory/person/${encodeURIComponent(personID)}`, operatorHeaders());
}

export async function recallCard(personID: string): Promise<RelationshipCard> {
  return apiGet<RelationshipCard>(`/api/people-memory/person/${encodeURIComponent(personID)}/card`, operatorHeaders());
}

export async function proposeCandidate(body: Partial<CaptureCandidate>): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>('/api/people-memory/propose', body, operatorHeaders());
}

export async function deferReceipt(body: {
  subject: string;
  person_id?: string;
  requester_dog?: string;
  source_coords?: SourceRef[];
}): Promise<DeferredPersonMemoryReceipt> {
  return apiPost<DeferredPersonMemoryReceipt>('/api/people-memory/defer', body, operatorHeaders());
}

export async function listCandidates(): Promise<CaptureCandidate[]> {
  return apiGet<CaptureCandidate[]>('/api/people-memory/candidates', operatorHeaders());
}

export async function getCandidate(id: string): Promise<CaptureCandidate> {
  return apiGet<CaptureCandidate>(`/api/people-memory/candidates/${encodeURIComponent(id)}`, operatorHeaders());
}

export async function approveCandidate(
  id: string,
  draftIDs: string[],
): Promise<PersonMemoryDecisionReceipt> {
  return apiPost<PersonMemoryDecisionReceipt>(
    `/api/people-memory/candidates/${encodeURIComponent(id)}/approve`,
    { draft_ids: draftIDs },
    operatorHeaders(),
  );
}

export async function rejectCandidate(id: string): Promise<{ status: string; candidate_id: string }> {
  return apiPost(`/api/people-memory/candidates/${encodeURIComponent(id)}/reject`, {}, operatorHeaders());
}

// rejectDrafts rejects individual drafts of a candidate (homologous
// per-card reject) — they are dropped and never materialized.
export async function rejectDrafts(id: string, draftIDs: string[]): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>(
    `/api/people-memory/candidates/${encodeURIComponent(id)}/reject-drafts`,
    { draft_ids: draftIDs },
    operatorHeaders(),
  );
}

export async function notNowCandidate(id: string): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>(`/api/people-memory/candidates/${encodeURIComponent(id)}/not-now`, {}, operatorHeaders());
}

export async function withdrawCandidate(id: string): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>(`/api/people-memory/candidates/${encodeURIComponent(id)}/withdraw`, {}, operatorHeaders());
}

export async function undoDecision(id: string, decisionID: string): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>(`/api/people-memory/candidates/${encodeURIComponent(id)}/undo`, {
    decision_id: decisionID,
  }, operatorHeaders());
}

export async function forgetProposal(id: string): Promise<PersonMemoryDeletionReceipt> {
  return apiPost<PersonMemoryDeletionReceipt>(
    `/api/people-memory/candidates/${encodeURIComponent(id)}/forget`,
    {},
    operatorHeaders(),
  );
}

export async function correctClaim(
  personID: string,
  claimID: string,
  payload: PersonClaimPayload,
  sourceRef: SourceRef,
): Promise<PersonClaimVersion> {
  return apiPost<PersonClaimVersion>(
    `/api/people-memory/person/${encodeURIComponent(personID)}/claims/${encodeURIComponent(claimID)}/correct`,
    { payload, source_ref: sourceRef },
    operatorHeaders(),
  );
}

export async function retireClaim(
  personID: string,
  claimID: string,
  sourceRef: SourceRef,
): Promise<{ status: string; claim_id: string }> {
  return apiPost(
    `/api/people-memory/person/${encodeURIComponent(personID)}/claims/${encodeURIComponent(claimID)}/retire`,
    { source_ref: sourceRef },
    operatorHeaders(),
  );
}

export async function amendEvent(
  personID: string,
  eventID: string,
  payload: CandidateInteractionDraft,
  sourceRef: SourceRef,
): Promise<InteractionEvent> {
  return apiPost<InteractionEvent>(
    `/api/people-memory/person/${encodeURIComponent(personID)}/events/${encodeURIComponent(eventID)}/amend`,
    { payload, source_ref: sourceRef },
    operatorHeaders(),
  );
}

export async function redactItem(
  personID: string,
  kind: 'claim' | 'event',
  id: string,
): Promise<{ status: string; kind: string; id: string }> {
  return apiPost(`/api/people-memory/person/${encodeURIComponent(personID)}/items/redact`, {
    kind,
    id,
  }, operatorHeaders());
}

export async function forgetPerson(personID: string): Promise<PersonMemoryDeletionReceipt> {
  return apiPost<PersonMemoryDeletionReceipt>(
    `/api/people-memory/person/${encodeURIComponent(personID)}/forget`,
    {},
    operatorHeaders(),
  );
}

export async function listDeferred(): Promise<DeferredPersonMemoryReceipt[]> {
  return apiGet<DeferredPersonMemoryReceipt[]>('/api/people-memory/deferred', operatorHeaders());
}

export async function claimDeferred(
  receiptID: string,
  requesterDog = 'operator',
): Promise<CaptureCandidate> {
  return apiPost<CaptureCandidate>(
    `/api/people-memory/deferred/${encodeURIComponent(receiptID)}/claim`,
    { requester_dog: requesterDog },
    operatorHeaders(),
  );
}

export async function withdrawReceipt(receiptID: string): Promise<{ status: string; receipt_id: string }> {
  return apiPost(`/api/people-memory/deferred/${encodeURIComponent(receiptID)}/withdraw`, {}, operatorHeaders());
}

export async function forgetReceipt(receiptID: string): Promise<{ status: string; receipt_id: string }> {
  return apiPost(`/api/people-memory/deferred/${encodeURIComponent(receiptID)}/forget`, {}, operatorHeaders());
}
