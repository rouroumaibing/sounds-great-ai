import { apiGet, apiPost } from './http';
import type { SessionRecordApi } from '../types/api';
import type { SessionRecord } from '../types';

function mapApiToSessionRecord(api: SessionRecordApi): SessionRecord {
  return {
    id: api.id,
    thread_id: api.thread_id,
    breed_id: api.breed_id,
    seq: api.seq,
    status: api.status,
    message_count: api.message_count,
    seal_reason: api.seal_reason,
    created_at: api.created_at,
    sealed_at: api.sealed_at ?? undefined,
    cliSessionId: api.id,
    dogId: api.breed_id,
  };
}

export const sessionService = {
  async getSessions(threadId: string): Promise<SessionRecord[]> {
    const data = await apiGet<SessionRecordApi[]>(`/api/threads/${threadId}/sessions`);
    return Array.isArray(data) ? data.map(mapApiToSessionRecord) : [];
  },

  async unsealSession(sessionId: string): Promise<void> {
    await apiPost(`/api/sessions/${sessionId}/unseal`, {});
  },
};
