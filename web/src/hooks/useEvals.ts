import { useCallback, useEffect, useState } from 'react';
import { apiGet, apiPost } from '../services/http';

export interface EvalDomainSummary {
  domain: {
    domainId: string;
    displayName: string;
    descriptionForHuman: string;
    evalBreed: string;
    frequency: string;
    enabled: boolean;
  };
  latestVerdict: {
    id: string;
    domainId: string;
    phenomenon: string;
    verdict: string;
    createdAt: string;
  } | null;
}

export function useEvals() {
  const [summaries, setSummaries] = useState<EvalDomainSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchEvals = useCallback(async () => {
    try {
      const data = await apiGet<EvalDomainSummary[]>('/api/evals');
      setSummaries(data);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchEvals();
    const interval = setInterval(fetchEvals, 60000); // auto-refresh 60s
    return () => clearInterval(interval);
  }, [fetchEvals]);

  const triggerRun = useCallback(async (domainId: string) => {
    await apiPost('/api/evals/run', { domainId });
    fetchEvals();
  }, [fetchEvals]);

  const appendLifecycle = useCallback(async (verdictId: string, event: { id: string; type: string; actor: string }) => {
    await apiPost(`/api/evals/results/${verdictId}/lifecycle`, event);
    fetchEvals();
  }, [fetchEvals]);

  return { summaries, loading, error, triggerRun, appendLifecycle, refresh: fetchEvals };
}
