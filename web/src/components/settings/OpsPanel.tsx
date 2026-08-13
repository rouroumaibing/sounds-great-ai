import { useEffect, useState } from 'react';
import {
  SettingsSection,
  SettingsFilterTabs,
  SettingsBadge,
  SettingsText,
  SettingsStatusStrip,
  SettingsRow,
} from './primitives';
import {
  getHealth,
  getMetricsText,
  getTraces,
  getEvals,
  parseMetricLines,
  type HealthInfo,
  type TraceSpan,
  type EvalSummary,
} from '../../services/opsService';

type SubTab = 'overview' | 'traces' | 'health' | 'eval';

const SUBTABS = [
  { key: 'overview', label: '总览' },
  { key: 'traces', label: 'Traces' },
  { key: 'health', label: '健康' },
  { key: 'eval', label: '评估' },
];

export function OpsPanel() {
  const [sub, setSub] = useState<SubTab>('overview');
  const [health, setHealth] = useState<HealthInfo | null>(null);
  const [metrics, setMetrics] = useState<{ name: string; value: string }[]>([]);
  const [spans, setSpans] = useState<TraceSpan[]>([]);
  const [evals, setEvals] = useState<EvalSummary[]>([]);
  const [error, setError] = useState('');

  // Overview + health are cheap and always relevant; load on mount.
  useEffect(() => {
    getHealth()
      .then(setHealth)
      .catch((e) => setError(String(e)));
    getMetricsText()
      .then((t) => setMetrics(parseMetricLines(t)))
      .catch(() => setMetrics([]));
  }, []);

  // Traces + evals load lazily when their tab is opened.
  useEffect(() => {
    if (sub === 'traces') {
      getTraces()
        .then((r) => setSpans(r.spans ?? []))
        .catch((e) => setError(String(e)));
    } else if (sub === 'eval') {
      getEvals()
        .then(setEvals)
        .catch((e) => setError(String(e)));
    }
  }, [sub]);

  return (
    <div className="space-y-6">
      <SettingsFilterTabs tabs={SUBTABS} activeKey={sub} onTabChange={(k) => setSub(k as SubTab)} />

      {error && (
        <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip>
      )}

      {sub === 'overview' && (
        <SettingsSection title="总览" description="运行态核心计数（来自 /api/ops/metrics）。">
          <div className="mt-1 grid grid-cols-2 gap-3 sm:grid-cols-4">
            {metrics.length === 0 ? (
              <SettingsText variant="xs" tone="muted" className="col-span-full">
                暂无指标数据。
              </SettingsText>
            ) : (
              metrics.slice(0, 12).map((m) => (
                <div key={m.name} className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
                  <SettingsText variant="xs" tone="muted" className="truncate font-mono">
                    {m.name}
                  </SettingsText>
                  <div className="mt-1 font-mono text-lg font-extrabold text-slate-100">{m.value}</div>
                </div>
              ))
            )}
          </div>
        </SettingsSection>
      )}

      {sub === 'traces' && (
        <SettingsSection title="Traces" description="最近一次采集窗口内的链路 span（来自 /api/ops/traces）。">
          <div className="mt-1 space-y-2">
            {spans.length === 0 ? (
              <SettingsText variant="xs" tone="muted">暂无 span。</SettingsText>
            ) : (
              spans.map((s) => (
                <SettingsRow
                  key={s.id}
                  title={<span className="font-mono text-xs">{s.name}</span>}
                  meta={s.id}
                  badges={
                    <SettingsBadge tone="emerald">OK</SettingsBadge>
                  }
                />
              ))
            )}
          </div>
        </SettingsSection>
      )}

      {sub === 'health' && (
        <SettingsSection title="健康" description="服务健康与运行时状态（来自 /api/ops/health）。">
          <div className="mt-1 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <SettingsText variant="xs" tone="muted">Status</SettingsText>
              <div className="mt-1 font-mono text-lg font-extrabold text-emerald-300">
                {health?.status ?? '—'}
              </div>
            </div>
            <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <SettingsText variant="xs" tone="muted">Uptime</SettingsText>
              <div className="mt-1 font-mono text-lg font-extrabold text-slate-100">
                {health?.uptime ?? '—'}
              </div>
            </div>
            <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <SettingsText variant="xs" tone="muted">OTel</SettingsText>
              <div className="mt-1 font-mono text-lg font-extrabold text-slate-100">
                {health?.otel?.status ?? '—'}
              </div>
            </div>
            <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <SettingsText variant="xs" tone="muted">Goroutines</SettingsText>
              <div className="mt-1 font-mono text-lg font-extrabold text-slate-100">
                {health?.goroutines ?? '—'}
              </div>
            </div>
          </div>
        </SettingsSection>
      )}

      {sub === 'eval' && (
        <SettingsSection title="评估" description="各评估域最新结论（来自 /api/evals）。">
          <div className="mt-1 space-y-2">
            {evals.length === 0 ? (
              <SettingsText variant="xs" tone="muted">暂无评估运行。</SettingsText>
            ) : (
              evals.map((e) => {
                const v = e.latestVerdict;
                const score = v && typeof v.score === 'number' ? v.score : null;
                return (
                  <SettingsRow
                    key={e.domain.domainId}
                    title={
                      <span>
                        {String(e.domain.displayName ?? e.domain.domainId)}
                        <SettingsText variant="micro" tone="muted" className="ml-2">
                          {e.domain.domainId}
                        </SettingsText>
                      </span>
                    }
                    meta={v?.phenomenon}
                    badges={
                      score != null ? (
                        <SettingsBadge tone="emerald">{score}</SettingsBadge>
                      ) : (
                        <SettingsBadge tone="slate">待跑</SettingsBadge>
                      )
                    }
                  />
                );
              })
            )}
          </div>
        </SettingsSection>
      )}
    </div>
  );
}
