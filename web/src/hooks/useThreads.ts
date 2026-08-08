import { useCallback, useEffect, useState } from 'react';
import { threadService } from '../services/threadService';
import { useAppStore } from '../store/useAppStore';
import type { Thread } from '../types';

export function useThreads() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const showToast = useAppStore((s) => s.showToast);

  const fetchThreads = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await threadService.getThreads();
      setThreads(data);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: `加载线程失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => {
    fetchThreads();
  }, [fetchThreads]);

  const createThread = useCallback(async (title: string) => {
    try {
      const thread = await threadService.createThread(title);
      setThreads((prev) => [thread, ...prev]);
      return thread;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `创建线程失败: ${msg}`, type: 'error' });
      throw e;
    }
  }, [showToast]);

  const deleteThread = useCallback(async (id: string) => {
    try {
      await threadService.deleteThread(id);
      setThreads((prev) => prev.filter((t) => t.id !== id));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `删除线程失败: ${msg}`, type: 'error' });
      throw e;
    }
  }, [showToast]);

  return { threads, loading, error, createThread, deleteThread, refetch: fetchThreads };
}
