import { apiGet, apiPost } from './http';
import type { MemoryEvidenceApi } from '../types/api';
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

export const memoryService = {
  async getEvidence(): Promise<SharedMemory[]> {
    const data = await apiGet<MemoryEvidenceApi[]>('/api/memory/evidence');
    return Array.isArray(data) ? data.map(mapEvidenceToSharedMemory) : [];
  },

  async addEvidence(content: string, _breed: string): Promise<SharedMemory> {
    const data = await apiPost<MemoryEvidenceApi>('/api/memory/evidence', { content, type: 'evidence', title: content.slice(0, 50) });
    return mapEvidenceToSharedMemory(data);
  },
};
