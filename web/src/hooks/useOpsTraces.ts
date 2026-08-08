import { useEffect, useState } from "react";
import { apiGet, ApiError } from "../services/http";

export interface OpsSpan {
  TraceID: string;
  SpanID: string;
  ParentID: string;
  Name: string;
  StartTime: string;
  EndTime: string;
  Attributes: Record<string, unknown>;
  Status: string;
}

export interface TraceQueryResult {
  spans: OpsSpan[];
  stats: { Count: number; MaxSize: number; Oldest: string } | null;
}

export function useOpsTraces(traceId?: string, breedId?: string): {
  traces: TraceQueryResult;
  loading: boolean;
  error: string | null;
} {
  const [traces, setTraces] = useState<TraceQueryResult>({ spans: [], stats: null });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const params = new URLSearchParams();
    if (traceId) params.set("traceId", traceId);
    if (breedId) params.set("breedId", breedId);
    params.set("limit", "100");
    apiGet<TraceQueryResult>(`/api/ops/traces?${params}`)
      .then((d) => {
        if (!cancelled) {
          setTraces({ spans: d.spans ?? [], stats: d.stats ?? null });
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [traceId, breedId]);

  return { traces, loading, error };
}
