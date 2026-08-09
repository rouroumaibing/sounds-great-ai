import clsx from 'clsx';
import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../services/http';
import { useI18n } from '../../store/useI18n';
import { OverviewTab } from '../ops/OverviewTab';
import { TracesTab } from '../ops/TracesTab';

interface HealthData {
  uptime: string;
  goroutines: number;
  mem_alloc: number;
  mem_total_alloc: number;
  mem_sys: number;
  mem_heap_alloc: number;
  mem_heap_sys: number;
  mem_heap_objects: number;
  mem_num_gc: number;
  status: string;
}

interface LogEntry {
  time: string;
  level: string;
  message: string;
}

interface GitData {
  branch: string;
  ahead: number;
  behind: number;
  dirty: boolean;
  untracked: number;
  modified: number;
}

type SubTab = 'overview' | 'metrics' | 'traces' | 'logs' | 'git';

export function OpsPanel() {
  const { t } = useI18n();
  const [subTab, setSubTab] = useState<SubTab>('overview');
  const [health, setHealth] = useState<HealthData | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [git, setGit] = useState<GitData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (subTab !== 'overview') return;
    apiGet<HealthData>('/api/ops/health')
      .then((data) => { setHealth(data); setError(null); })
      .catch((e: ApiError) => setError(e.message));
  }, [subTab]);

  useEffect(() => {
    if (subTab !== 'logs') return;
    const fetchLogs = () => {
      apiGet<LogEntry[]>('/api/ops/logs')
        .then((data) => { setLogs(data); setError(null); })
        .catch((e: ApiError) => setError(e.message));
    };
    fetchLogs();
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
  }, [subTab]);

  useEffect(() => {
    if (subTab !== 'git') return;
    apiGet<GitData>('/api/ops/git')
      .then((data) => { setGit(data); setError(null); })
      .catch((e: ApiError) => setError(e.message));
  }, [subTab]);

  const subTabs: { id: SubTab; label: string; icon: string }[] = [
    { id: 'overview', label: t('ops.overview'), icon: 'fa-solid fa-gauge' },
    { id: 'metrics', label: t('ops.metrics'), icon: 'fa-solid fa-chart-line' },
    { id: 'traces', label: t('ops.traces'), icon: 'fa-solid fa-route' },
    { id: 'logs', label: t('ops.logs'), icon: 'fa-solid fa-list' },
    { id: 'git', label: 'Git', icon: 'fa-solid fa-code-branch' },
  ];

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">{t('ops.title')}</h2>
        <p className="text-xs text-slate-400 mt-1">{t('ops.desc')}</p>
      </div>

      <div className="flex space-x-1 border-b border-slate-800/80">
        {subTabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setSubTab(tab.id)}
            className={clsx(
              'px-4 py-2 text-xs font-semibold border-b-2 transition',
              subTab === tab.id
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-500 hover:text-slate-300'
            )}
          >
            <i className={clsx(tab.icon, 'mr-1.5')}></i>
            {tab.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="text-center text-rose-400 py-4">{t('common.error')}: {error}</div>
      )}

      {subTab === 'overview' && health && (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
          <StatCard label={t('ops.uptime')} value={health.uptime} icon="fa-solid fa-clock" />
          <StatCard label="Goroutines" value={String(health.goroutines ?? 0)} icon="fa-solid fa-microchip" />
          <StatCard label={t('ops.status')} value={health.status} icon="fa-solid fa-heart-pulse" valueClass="text-emerald-400" />
          <StatCard label={t('ops.memAlloc')} value={(Number(health.mem_alloc) / 1048576).toFixed(2)} icon="fa-solid fa-memory" />
          <StatCard label={t('ops.memSys')} value={(Number(health.mem_sys) / 1048576).toFixed(2)} icon="fa-solid fa-server" />
          <StatCard label="GC Count" value={String(health.mem_num_gc ?? 0)} icon="fa-solid fa-broom" />
          <StatCard label="Heap Objects" value={String(health.mem_heap_objects ?? 0)} icon="fa-solid fa-comments" />
        </div>
      )}

      {subTab === 'metrics' && <OverviewTab />}

      {subTab === 'traces' && <TracesTab />}

      {subTab === 'logs' && (
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
          <div className="px-4 py-2 border-b border-slate-800/80 text-[11px] text-slate-400 flex items-center justify-between">
            <span>{t('ops.recentLogs').replace('{count}', String(logs.length))}</span>
            <i className="fa-solid fa-arrows-rotate text-slate-500"></i>
          </div>
          <div className="max-h-[500px] overflow-y-auto">
            {logs.length === 0 ? (
              <div className="text-center text-slate-500 py-8 text-xs">{t('ops.noLogs')}</div>
            ) : (
              logs.slice().reverse().map((log, i) => (
                <div key={i} className="px-4 py-2 border-b border-slate-800/40 text-xs font-mono flex items-start space-x-3">
                  <span className="text-slate-500 shrink-0">{log.time}</span>
                  <span className={clsx('shrink-0 font-bold w-12', log.level === 'error' ? 'text-rose-400' : log.level === 'warn' ? 'text-amber-400' : 'text-slate-400')}>
                    {log.level}
                  </span>
                  <span className="text-slate-300 break-all">{log.message}</span>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {subTab === 'git' && git && (
        <div className="space-y-4">
          <div className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800/80">
            <div className="flex items-center space-x-3 mb-4">
              <i className="fa-solid fa-code-branch text-indigo-400"></i>
              <span className="text-sm font-bold text-slate-100 font-mono">{git.branch || '(unknown)'}</span>
              {git.dirty && (
                <span className="px-2 py-0.5 rounded-lg bg-amber-500/20 text-amber-400 border border-amber-500/30 text-[10px] font-bold">
                  DIRTY
                </span>
              )}
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <StatCard label="Ahead" value={String(git.ahead)} icon="fa-solid fa-arrow-up" />
              <StatCard label="Behind" value={String(git.behind)} icon="fa-solid fa-arrow-down" />
              <StatCard label="Untracked" value={String(git.untracked)} icon="fa-solid fa-question" />
              <StatCard label="Modified" value={String(git.modified)} icon="fa-solid fa-pen" />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function StatCard({ label, value, icon, valueClass }: { label: string; value: string; icon: string; valueClass?: string }) {
  return (
    <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800/80">
      <div className="flex items-center space-x-2 mb-1">
        <i className={clsx(icon, 'text-slate-500 text-[10px]')}></i>
        <span className="text-[10px] text-slate-400 font-semibold">{label}</span>
      </div>
      <span className={clsx('text-sm font-mono font-bold', valueClass ?? 'text-slate-200')}>{value}</span>
    </div>
  );
}
