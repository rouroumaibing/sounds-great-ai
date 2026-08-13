import { useCallback, useEffect, useState } from 'react';
import { fetchTrail, type Briefing } from '../services/custody';

interface UseCustodyTrailResult {
  briefing: Briefing | null;
  loading: boolean;
  error: string | null;
  refresh: () => void;
}

// useCustodyTrail loads the ball-custody briefing for a thread and re-fetches
// whenever the thread changes. A manual `refresh()` is also exposed for the
// UI refresh button.
export function useCustodyTrail(threadId: string | undefined): UseCustodyTrailResult {
  const [briefing, setBriefing] = useState<Briefing | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    if (!threadId) return;
    setLoading(true);
    setError(null);
    fetchTrail(threadId)
      .then(setBriefing)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [threadId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { briefing, loading, error, refresh };
}
