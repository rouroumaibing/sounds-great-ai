import { apiGet, apiPost } from './http';
import type { MemoryEvidenceApi, LaneEntryApi, RecallEventApi, RecallLedgerApi, ReflectRequestApi, ReflectResponseApi, LaneEdgeApi, LaneMarkerApi, SensitivityLevel, LifecycleTraceApi } from '../types/api';
import type { SharedMemory } from '../types';

function mapEvidenceToSharedMemory(e: MemoryEvidenceApi): SharedMemory {
  return {
    id: 0,
    type: e.type.toUpperCase(),
    time: new Date(e.created_at).toLocaleTimeString('zh-CN', { hour12: false }),
    fact: e.content,
    author: e.thread_id || 'system',
  };
}

// Human disposition action on a lane candidate (maps to the backend
// /api/memory/lanes/{id}/{action} endpoints).
export type LaneDisposition = 'approve' | 'reject' | 'modify' | 'retire' | 'forget' | 'defer' | 'undo' | 'withdraw';

// Recall consumption outcome (matches backend RecallOutcome*).
export type RecallOutcome = 'used' | 'ignored';

export const memoryService = {
  async getEvidence(): Promise<SharedMemory[]> {
    const data = await apiGet<MemoryEvidenceApi[]>('/api/memory/evidence');
    return Array.isArray(data) ? data.map(mapEvidenceToSharedMemory) : [];
  },

  async addEvidence(content: string, _breed: string): Promise<SharedMemory> {
    const data = await apiPost<MemoryEvidenceApi>('/api/memory/evidence', { content, type: 'evidence', title: content.slice(0, 50) });
    return mapEvidenceToSharedMemory(data);
  },

  // --- Typed-lane Shared Memory (P3) ---

  async getLanesPending(): Promise<LaneEntryApi[]> {
    const data = await apiGet<LaneEntryApi[]>('/api/memory/lanes/pending');
    return Array.isArray(data) ? data : [];
  },

  async getLanesTruth(lane?: string): Promise<LaneEntryApi[]> {
    const q = lane ? `?lane=${encodeURIComponent(lane)}` : '';
    const data = await apiGet<LaneEntryApi[]>(`/api/memory/lanes/truth${q}`);
    return Array.isArray(data) ? data : [];
  },

  async disposeLane(id: string, action: LaneDisposition, content?: string): Promise<LaneEntryApi> {
    if (action === 'modify') {
      return apiPost<LaneEntryApi>(`/api/memory/lanes/${id}/modify`, { content });
    }
    return apiPost<LaneEntryApi>(`/api/memory/lanes/${id}/${action}`, {});
  },

  // --- Recall observability (RecallFeed / RecallLedger) ---

  async getRecallEvents(limit = 20): Promise<RecallEventApi[]> {
    const data = await apiGet<RecallEventApi[]>(`/api/memory/lanes/recall/events?limit=${limit}`);
    return Array.isArray(data) ? data : [];
  },

  async getRecallLedger(windows = '7,14,30'): Promise<RecallLedgerApi> {
    const data = await apiGet<RecallLedgerApi>(`/api/memory/lanes/recall/ledger?windows=${encodeURIComponent(windows)}`);
    return data && typeof data === 'object' ? data : {};
  },

  // Mark the consumption outcome of a recall event (used/ignored), completing
  // the consumption-verification loop (P0-1, homologous clowder RecallFeed).
  async markRecallOutcome(id: string, outcome: RecallOutcome): Promise<void> {
    await apiPost<void>(`/api/memory/lanes/recall/${id}/outcome`, { outcome });
  },

  // On-demand "pull" recall: surface approved truth matching a query (or all
  // approved truth when query is empty), homologous clowder RecallFeed pull mode.
  async pullRecall(query = ''): Promise<{ count: number; entry_ids: string[]; entries: LaneEntryApi[] }> {
    const q = query ? `?query=${encodeURIComponent(query)}` : '';
    return apiPost<{ count: number; entry_ids: string[]; entries: LaneEntryApi[] }>(`/api/memory/lanes/recall/pull${q}`, {});
  },

  // Full-text search over lane content (FTS5, P1-5).
  async searchLanes(query: string): Promise<LaneEntryApi[]> {
    const data = await apiGet<LaneEntryApi[]>(`/api/memory/lanes/search?q=${encodeURIComponent(query)}`);
    return Array.isArray(data) ? data : [];
  },

  // LLM abstractive reflection over approved truth (P2-6, sanctioned synthesis
  // service). Output is returned, not auto-committed; with seed=true it becomes
  // a PENDING candidate that still requires human disposition (M5 提交权).
  async reflectLanes(opts: ReflectRequestApi = {}): Promise<ReflectResponseApi> {
    return apiPost<ReflectResponseApi>('/api/memory/lanes/reflect', opts);
  },

  // Gap1: link two entries with a typed relation + optional edge sensitivity /
  // provenance (clowder V18 edge columns). operator scopes the write to the
  // acting dog (requestOperator: X-Operator > ?operator= > default).
  async linkEntries(from: string, to: string, relation: string, edgeSensitivity = '', provenance = 'manual', operator = ''): Promise<LaneEdgeApi> {
    const q = operator ? `?operator=${encodeURIComponent(operator)}` : '';
    return apiPost<LaneEdgeApi>(`/api/memory/lanes/${from}/link${q}`, { to, relation, edge_sensitivity: edgeSensitivity, provenance });
  },

  // Gap1: attach a normalized marker to an entry (clowder marker).
  async markEntry(id: string, markerType: string, content: string): Promise<LaneMarkerApi> {
    return apiPost<LaneMarkerApi>(`/api/memory/lanes/${id}/mark`, { marker_type: markerType, content });
  },

  // Gap1: read the outgoing edges + markers around an entry (relationship graph).
  async getGraph(id: string): Promise<{ edges: LaneEdgeApi[]; markers: LaneMarkerApi[] }> {
    return apiGet<{ edges: LaneEdgeApi[]; markers: LaneMarkerApi[] }>(`/api/memory/lanes/${id}/graph`);
  },

  // Gap2: re-classify an entry's sensitivity level (4-tier ACL enforcement).
  // When widening visibility (e.g. private→public) the backend returns 409
  // unless confirmWidening=true (Task #33, homologous confirmVisibilityWidening).
  async setSensitivity(id: string, sensitivity: SensitivityLevel, confirmWidening = false, operator = ''): Promise<{ status?: string; error?: string; confirm_field?: string; current?: string; requested?: string }> {
    const q = operator ? `?operator=${encodeURIComponent(operator)}` : '';
    return apiPost<{ status?: string; error?: string; confirm_field?: string; current?: string; requested?: string }>(
      `/api/memory/lanes/${id}/sensitivity${q}`,
      { sensitivity, confirm_visibility_widening: confirmWidening },
    );
  },

  // Gap3: dense-vector semantic recall over approved truth (opt-in embedder).
  async semanticSearch(query: string, topK = 10): Promise<LaneEntryApi[]> {
    const data = await apiPost<LaneEntryApi[]>('/api/memory/lanes/search/semantic', { query, top_k: topK });
    return Array.isArray(data) ? data : [];
  },

  // Gap3: rebuild the dense-vector index from all approved truth (idempotent).
  async reindexVectors(): Promise<{ status: string }> {
    return apiPost<{ status: string }>('/api/memory/lanes/reindex', {});
  },

  // P1 three-axis: append-only lifecycle traces (creation/consumption/correction).
  async getLifecycle(limit = 50): Promise<LifecycleTraceApi[]> {
    const data = await apiGet<LifecycleTraceApi[]>(`/api/memory/lanes/lifecycle?limit=${limit}`);
    return Array.isArray(data) ? data : [];
  },

  // Mark the three-axis outcome of a recall event (used/ignored + optional axis/maturity).
  // operator re-affirms the acting dog who confirmed the outcome (multi-operator attribution).
  async markRecallOutcomeDetailed(id: string, outcome: RecallOutcome, axis = '', maturity = '', operator = ''): Promise<void> {
    const q = operator ? `?operator=${encodeURIComponent(operator)}` : '';
    await apiPost<void>(`/api/memory/lanes/recall/${id}/outcome${q}`, { outcome, axis, maturity });
  },
};
