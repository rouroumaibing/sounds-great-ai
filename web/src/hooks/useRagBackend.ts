import { useCallback, useEffect, useRef, useState } from 'react';
import { ragService } from '../services/ragService';
import { useAppStore } from '../store/useAppStore';
import type { RagBackendApi, SyncProgressApi } from '../types/api';

export function useRagBackend() {
  const [backend, setBackend] = useState<RagBackendApi | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncProgress, setSyncProgress] = useState<SyncProgressApi | null>(null);
  const [switching, setSwitching] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const showToast = useAppStore((s) => s.showToast);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchBackend = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await ragService.getBackend();
      setBackend(data);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: `加载RAG后端失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchBackend(); }, [fetchBackend]);

  useEffect(() => {
    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, []);

  const switchBackend = useCallback(async (backendName: string) => {
    setSwitching(true);
    try {
      await ragService.switchBackend(backendName);
      showToast({ message: `已切换到 ${backendName}`, type: 'success' });
      await fetchBackend();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `切换后端失败: ${msg}`, type: 'error' });
    } finally {
      setSwitching(false);
    }
  }, [fetchBackend, showToast]);

  const triggerSync = useCallback(async (from: string) => {
    if (!backend) return;
    const to = backend.active;
    setSyncing(true);
    setSyncProgress(null);
    try {
      await ragService.triggerSync(from);
      if (pollRef.current) {
        clearInterval(pollRef.current);
        pollRef.current = null;
      }
      pollRef.current = setInterval(async () => {
        try {
          const progress = await ragService.getSyncProgress(from, to);
          setSyncProgress(progress);
          if (progress.status === 'completed') {
            if (pollRef.current) {
              clearInterval(pollRef.current);
              pollRef.current = null;
            }
            setSyncing(false);
            showToast({ message: '同步完成', type: 'success' });
          } else if (progress.status === 'error') {
            if (pollRef.current) {
              clearInterval(pollRef.current);
              pollRef.current = null;
            }
            setSyncing(false);
            showToast({ message: `同步失败: ${progress.error ?? 'unknown'}`, type: 'error' });
          }
        } catch {
          if (pollRef.current) {
            clearInterval(pollRef.current);
            pollRef.current = null;
          }
          setSyncing(false);
          showToast({ message: '同步进度查询失败', type: 'error' });
        }
      }, 2000);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `触发同步失败: ${msg}`, type: 'error' });
      setSyncing(false);
    }
  }, [backend, showToast]);

  return { backend, loading, error, syncProgress, switching, syncing, switchBackend, triggerSync, refetch: fetchBackend };
}
