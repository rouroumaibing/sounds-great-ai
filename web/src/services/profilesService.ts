import { apiGet, apiPost, apiPut, ApiError } from '../services/http';
import type {
  CapsuleSummary,
  RelationshipCapsule,
  DistillResult,
  DistillAgentResult,
} from '../types/profiles';

function enc(key: string): string {
  return encodeURIComponent(key);
}

export async function listCapsules(): Promise<CapsuleSummary[]> {
  return apiGet<CapsuleSummary[]>('/api/profiles');
}

export async function getCapsule(key: string): Promise<RelationshipCapsule> {
  return apiGet<RelationshipCapsule>(`/api/profiles/${enc(key)}`);
}

export async function getProposal(key: string): Promise<RelationshipCapsule | null> {
  try {
    return await apiGet<RelationshipCapsule>(`/api/profiles/${enc(key)}/proposal`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

export async function approveProposal(key: string): Promise<RelationshipCapsule> {
  return apiPost<RelationshipCapsule>(`/api/profiles/${enc(key)}/proposal/approve`, {});
}

export async function rejectProposal(key: string): Promise<{ status: string; relationship_key: string; eval_rejections: number }> {
  return apiPost(`/api/profiles/${enc(key)}/proposal/reject`, {});
}

export async function distill(key: string): Promise<DistillResult> {
  return apiPost<DistillResult>(`/api/profiles/${enc(key)}/distill`, {});
}

// Autonomous distill. The distiller is derived from the
// CURRENT session via sessionId (no hardcoded default dog); clientId is an
// explicit operator override. Exactly one of the two must be supplied or the
// backend refuses with 400.
export async function distillAgent(
  key: string,
  opts: { sessionId?: string; clientId?: string },
): Promise<DistillAgentResult> {
  const params = new URLSearchParams();
  if (opts.sessionId) params.set('session_id', opts.sessionId);
  else if (opts.clientId) params.set('client_id', opts.clientId);
  const qs = params.toString();
  return apiPost<DistillAgentResult>(
    `/api/profiles/${enc(key)}/distill/agent${qs ? `?${qs}` : ''}`,
    {},
  );
}

export async function upsertCapsule(key: string, body: string, ownerDog = 'operator'): Promise<RelationshipCapsule> {
  return apiPut<RelationshipCapsule>(`/api/profiles/${enc(key)}`, { body, owner_dog: ownerDog });
}
