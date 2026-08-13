import { API_BASE, apiGet, authHeaders } from './http';

export interface HealthInfo {
  status: string;
  uptime: string;
  goroutines: number;
  otel: { status: string; tracesEnabled: boolean; metricsEnabled: boolean };
  [key: string]: unknown;
}

export interface TraceSpan {
  id: string;
  name: string;
  startTime: string;
  endTime?: string;
  attributes?: Record<string, string>;
}

export interface TracesResponse {
  spans: TraceSpan[];
  stats: unknown;
}

export interface EvalSummary {
  domain: { domainId: string; displayName?: string; [key: string]: unknown };
  latestVerdict?: {
    id: string;
    domainId: string;
    verdict: string;
    phenomenon: string;
    [key: string]: unknown;
  };
}

export async function getHealth(): Promise<HealthInfo> {
  return apiGet<HealthInfo>('/api/ops/health');
}

// getMetricsText returns the raw Prometheus text snapshot from /api/ops/metrics.
export async function getMetricsText(): Promise<string> {
  const res = await fetch(`${API_BASE}/api/ops/metrics`, { headers: authHeaders() });
  return res.text();
}

export async function getTraces(): Promise<TracesResponse> {
  return apiGet<TracesResponse>('/api/ops/traces');
}

export async function getEvals(): Promise<EvalSummary[]> {
  return apiGet<EvalSummary[]>('/api/evals');
}

// parseMetricLines extracts `name value` (and `name{labels} value`) pairs from
// Prometheus text so the overview can render real counters without hardcoding
// SG-specific metric names.
export function parseMetricLines(text: string): { name: string; value: string }[] {
  const out: { name: string; value: string }[] = [];
  const re = /^([a-zA-Z_][\w]*)(?:\{[^}]*\})?\s+([\d.eE+-]+)/gm;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    out.push({ name: m[1], value: m[2] });
  }
  return out;
}
