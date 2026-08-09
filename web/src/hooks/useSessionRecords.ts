import { useCallback, useEffect, useState } from 'react';
import { sessionService } from '../services/sessionService';
import { useAppStore } from '../store/useAppStore';
import type { SessionRecord } from '../types';
import { useI18n } from '../store/useI18n';

export function useSessionRecords(threadId: string | null) {
  const [sessions, setSessions] = useState<SessionRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { t } = useI18n();
  const showToast = useAppStore((s) => s.showToast);

  const fetchSessions = useCallback(async () => {
    if (!threadId) {
      setSessions([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await sessionService.getSessions(threadId);
      setSessions(data);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: t('hooks.usesessionrecords.s1').replace('{msg}', msg), type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [threadId, showToast]);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);

  const unseal = useCallback(async (sessionId: string) => {
    try {
      await sessionService.unsealSession(sessionId);
      showToast({ message: t('hooks.usesessionrecords.s2'), type: 'success' });
      await fetchSessions();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usesessionrecords.s3').replace('{msg}', msg), type: 'error' });
    }
  }, [fetchSessions, showToast]);

  return { sessions, loading, error, unseal, refetch: fetchSessions };
}
