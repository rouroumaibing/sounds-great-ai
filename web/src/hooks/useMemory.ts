import { useCallback, useEffect, useState } from 'react';
import { memoryService } from '../services/memoryService';
import { useAppStore } from '../store/useAppStore';
import type { SharedMemory } from '../types';

export function useMemory() {
  const [memories, setMemories] = useState<SharedMemory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const showToast = useAppStore((s) => s.showToast);

  const fetchMemories = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await memoryService.getEvidence();
      setMemories(data);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: `加载记忆失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchMemories(); }, [fetchMemories]);

  return { memories, loading, error, refetch: fetchMemories };
}
