import { apiGet, apiPost } from './http';
import type { RagBackendApi, SyncProgressApi } from '../types/api';

export const ragService = {
  async getBackend(): Promise<RagBackendApi> {
    return await apiGet<RagBackendApi>('/api/rag/backend');
  },

  async switchBackend(backend: string): Promise<void> {
    await apiPost('/api/rag/backend/switch', { backend });
  },

  async triggerSync(from: string): Promise<void> {
    await apiPost('/api/rag/sync', { from });
  },

  async getSyncProgress(from: string, to: string): Promise<SyncProgressApi> {
    return await apiGet<SyncProgressApi>(`/api/rag/sync/progress?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`);
  },
};
