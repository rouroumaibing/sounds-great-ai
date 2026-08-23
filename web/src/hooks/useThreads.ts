import { useCallback, useEffect, useState } from 'react';
import { threadService } from '../services/threadService';
import { useAppStore } from '../store/useAppStore';
import type { Thread } from '../types';
import { useI18n } from '../store/useI18n';

export function useThreads() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { t } = useI18n();
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
      showToast({ message: t('hooks.usethreads.s1').replace('{msg}', msg), type: 'error' });
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
      showToast({ message: t('hooks.usethreads.s2').replace('{msg}', msg), type: 'error' });
      throw e;
    }
  }, [showToast]);

  const deleteThread = useCallback(async (id: string) => {
    try {
      await threadService.deleteThread(id);
      setThreads((prev) => prev.filter((t) => t.id !== id));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usethreads.s3').replace('{msg}', msg), type: 'error' });
      throw e;
    }
  }, [showToast]);

  const renameThread = useCallback(async (id: string, title: string) => {
    try {
      await threadService.renameThread(id, title);
      setThreads((prev) => prev.map((t) => (t.id === id ? { ...t, title } : t)));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usethreads.s4').replace('{msg}', msg), type: 'error' });
      throw e;
    }
  }, [showToast]);

  return { threads, loading, error, createThread, deleteThread, renameThread, refetch: fetchThreads };
}
