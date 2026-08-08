import { apiGet, apiPost, apiDelete } from './http';
import type { Thread, StreamEvent } from '../types';

export const threadService = {
  async getThreads(): Promise<Thread[]> {
    const data = await apiGet<Thread[]>('/api/threads');
    return Array.isArray(data) ? data : [];
  },

  async createThread(title: string): Promise<Thread> {
    return apiPost<Thread>('/api/threads', { title });
  },

  async deleteThread(id: string): Promise<void> {
    await apiDelete(`/api/threads/${id}`);
  },

  async getThreadEvents(id: string): Promise<StreamEvent[]> {
    const data = await apiGet<{ events: StreamEvent[] }>(`/api/threads/${id}`);
    return Array.isArray(data?.events) ? data.events : [];
  },
};
