import { useEffect, useState, useCallback } from "react";
import { apiGet, ApiError } from "../services/http";

export interface MetricsSnapshot {
  timestamp: string;
  text: string;
}

export function useOpsMetrics(): {
  snapshots: MetricsSnapshot[];
  loading: boolean;
  error: string | null;
  refresh: () => void;
} {
  const [snapshots, setSnapshots] = useState<MetricsSnapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiGet<MetricsSnapshot[]>("/api/ops/metrics/history");
      setSnapshots(data ?? []);
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 30_000);
    return () => clearInterval(id);
  }, [refresh]);

  return { snapshots, loading, error, refresh };
}
