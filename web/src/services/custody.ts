import { apiGet } from './http';

// Brief & Trail API read models (mirrors internal/domains/custody/ports/briefing.go).
export interface TrailEntry {
  seq: number;
  type: string;
  holder?: string;
  from?: string;
  to?: string;
  timestamp: number;
}

export interface Briefing {
  thread_id: string;
  state: string;
  holder?: string;
  turns: number;
  handoffs: number;
  holds: number;
  trail: TrailEntry[];
}

// Fetch the custody briefing/trail for a thread from the Brief & Trail API
// (P4 engine: GET /api/custody/threads/{threadID}/trail).
export function fetchTrail(threadId: string): Promise<Briefing> {
  return apiGet<Briefing>(`/api/custody/threads/${encodeURIComponent(threadId)}/trail`);
}
