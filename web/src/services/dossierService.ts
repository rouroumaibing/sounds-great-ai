import { apiGet, apiPost } from '../services/http';
import type {
  DossierObservation,
  DossierOverview,
  DistillationProposal,
} from '../types/dossier';

export async function getDossierOverview(): Promise<DossierOverview> {
  return apiGet<DossierOverview>('/api/dossier');
}

export async function listObservations(dogId?: string): Promise<Record<string, DossierObservation[]> | DossierObservation[]> {
  const q = dogId ? `?dogId=${encodeURIComponent(dogId)}` : '';
  const res = await apiGet<{ observations: Record<string, DossierObservation[]> | DossierObservation[] }>(`/api/dossier/observations${q}`);
  return res.observations ?? {};
}

export async function addObservation(dogId: string, content: string): Promise<DossierObservation> {
  const res = await apiPost<{ observation: DossierObservation }>('/api/dossier/observations', { dogId, content });
  return res.observation;
}

export async function listProposals(params?: { dogId?: string; status?: string }): Promise<DistillationProposal[]> {
  const qs = new URLSearchParams();
  if (params?.dogId) qs.set('dogId', params.dogId);
  const suffix = qs.toString() ? `?${qs.toString()}` : '';
  const res = await apiGet<{ proposals: DistillationProposal[] }>(`/api/dossier/distillations${suffix}`);
  return res.proposals ?? [];
}

export async function approveProposal(proposalId: string): Promise<DistillationProposal> {
  const res = await apiPost<{ proposal: DistillationProposal }>(`/api/dossier/distillations/${encodeURIComponent(proposalId)}/approve`, {});
  return res.proposal;
}

export async function rejectProposal(proposalId: string, reason: string): Promise<DistillationProposal> {
  const res = await apiPost<{ proposal: DistillationProposal }>(`/api/dossier/distillations/${encodeURIComponent(proposalId)}/reject`, { reason });
  return res.proposal;
}

export async function executeApply(proposalId: string, actor: string): Promise<{ proposal: DistillationProposal; commitSha: string }> {
  return apiPost<{ proposal: DistillationProposal; commitSha: string }>(`/api/dossier/distillations/${encodeURIComponent(proposalId)}/execute-apply`, { actor });
}
