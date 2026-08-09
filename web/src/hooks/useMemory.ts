import { useCallback, useEffect, useState } from 'react';
import { memoryService } from '../services/memoryService';
import { useAppStore } from '../store/useAppStore';
import type { SharedMemory } from '../types';
import { useI18n } from '../store/useI18n';

export function useMemory() {
  const [memories, setMemories] = useState<SharedMemory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { t } = useI18n();
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
      showToast({ message: t('hooks.usememory.s1').replace('{msg}', msg), type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchMemories(); }, [fetchMemories]);

  return { memories, loading, error, refetch: fetchMemories };
}
