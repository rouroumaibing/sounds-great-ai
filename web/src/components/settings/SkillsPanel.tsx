import { useCallback, useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet, apiPatch, apiPost } from '../../services/http';
import type { SkillItem, SkillDetail, SkillDriftIssue } from '../../types';

const CARRIERS = ['claude', 'codex', 'gemini', 'opencode', 'kimi'] as const;

const DRIFT_LABEL: Record<string, string> = {
  unregistered: '未启用',
  phantom: '源已缺失',
  conflict: '目录冲突',
  'mount-missing': '挂载缺失',
  'stale-mount': '过期挂载',
};

const HEALTH_LABEL: Record<string, string> = {
  disabled: '已禁用',
  missing: '挂载缺失',
  mounted: '已挂载',
  logical: '逻辑挂载',
};

const HEALTH_COLOR: Record<string, string> = {
  disabled: 'text-slate-500 bg-slate-800/60',
  missing: 'text-rose-400 bg-rose-500/15',
  mounted: 'text-emerald-400 bg-emerald-500/15',
  logical: 'text-sky-400 bg-sky-500/15',
};

const SECURITY_LABEL: Record<string, string> = {
  approved: '已批准',
  pending: '待批准',
  quarantined: '已隔离',
  revoked: '已撤销',
};

const SECURITY_COLOR: Record<string, string> = {
  approved: 'text-emerald-400 bg-emerald-500/15',
  pending: 'text-amber-400 bg-amber-500/15',
  quarantined: 'text-rose-400 bg-rose-500/15',
  revoked: 'text-slate-400 bg-slate-700/40',
};

export function SkillsPanel() {
  const [items, setItems] = useState<SkillItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [drift, setDrift] = useState<SkillDriftIssue[] | null>(null);
  const [driftBusy, setDriftBusy] = useState(false);
  const [preview, setPreview] = useState<SkillDetail | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [driftStrategy, setDriftStrategy] = useState<'keep-project' | 'use-global'>('keep-project');

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiGet<SkillItem[]>('/api/skills');
      setItems(Array.isArray(data) ? data : []);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const patchSkill = useCallback(
    async (id: string, body: Record<string, unknown>) => {
      try {
        const updated = await apiPatch<SkillItem>(`/api/skills/${id}`, body);
        setItems((prev) => prev.map((it) => (it.id === id ? updated : it)));
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    },
    [],
  );

  const toggleEnabled = (it: SkillItem) =>
    patchSkill(it.id, { enabled: !it.enabled, mountPoints: it.mountPoints });

  const toggleCarrier = (it: SkillItem, carrier: string) => {
    const set = new Set(it.mountPoints);
    if (set.has(carrier)) set.delete(carrier);
    else set.add(carrier);
    patchSkill(it.id, { mountPoints: Array.from(set) });
  };

  const checkDrift = async () => {
    setDriftBusy(true);
    try {
      const res = await apiPost<{ issues: SkillDriftIssue[] }>('/api/skills/drift/check', {});
      setDrift(res.issues ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setDriftBusy(false);
    }
  };

  const resolveDrift = async () => {
    setDriftBusy(true);
    try {
      const res = await apiPost<{ issues: SkillDriftIssue[] }>(
        '/api/skills/drift/resolve',
        { strategy: driftStrategy },
      );
      setDrift(res.issues ?? []);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setDriftBusy(false);
    }
  };

  const syncMounts = async () => {
    setSyncing(true);
    try {
      await apiPost('/api/skills/sync', {});
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSyncing(false);
    }
  };

  const openPreview = async (id: string) => {
    try {
      const detail = await apiGet<SkillDetail>(`/api/skills/${id}`);
      setPreview(detail);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  };

  const securityAction = useCallback(
    async (id: string, action: 'approve' | 'quarantine' | 'revoke') => {
      try {
        await apiPost<SkillItem>(`/api/skills/security/${id}/${action}`, {});
        await load();
        if (preview && preview.id === id) {
          const detail = await apiGet<SkillDetail>(`/api/skills/${id}`);
          setPreview(detail);
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    },
    [load, preview],
  );

  const filtered = items.filter((it) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return (
      it.name.toLowerCase().includes(q) ||
      it.id.toLowerCase().includes(q) ||
      (it.category || '').toLowerCase().includes(q) ||
      (it.description || '').toLowerCase().includes(q)
    );
  });

  const enabledCount = items.filter((it) => it.enabled).length;

  return (
    <div className="space-y-4">
      {/* 概览 + 漂移治理 */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-300 font-mono">
          共 {items.length} 项 · 启用 {enabledCount}
        </div>
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索技能名 / 分类 / 描述"
          className="flex-1 min-w-[180px] px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-200 placeholder:text-slate-600 focus:outline-none focus:border-indigo-500"
        />
        <button
          onClick={checkDrift}
          disabled={driftBusy}
          className={clsx('px-3 py-1.5 rounded-lg text-xs font-mono transition', driftBusy ? 'bg-slate-800 text-slate-500' : 'bg-indigo-600 hover:bg-indigo-500 text-white')}
        >
          {driftBusy ? '检测中…' : '检查漂移'}
        </button>
        <button
          onClick={syncMounts}
          disabled={syncing}
          className={clsx('px-3 py-1.5 rounded-lg text-xs font-mono transition', syncing ? 'bg-slate-800 text-slate-500' : 'bg-slate-700 hover:bg-slate-600 text-white')}
        >
          {syncing ? '同步中…' : '同步挂载'}
        </button>
      </div>

      {drift && drift.length > 0 && (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-amber-300">
              检测到 {drift.length} 处漂移
            </span>
            <div className="flex items-center gap-1">
              <div className="flex items-center gap-1">
                {(['keep-project', 'use-global'] as const).map((st) => (
                  <button
                    key={st}
                    onClick={() => setDriftStrategy(st)}
                    className={clsx('px-2 py-0.5 rounded text-[10px] font-mono transition', driftStrategy === st ? 'bg-amber-600 text-white' : 'bg-slate-800 text-slate-400 hover:bg-slate-700')}
                  >
                    {st === 'keep-project' ? '保项目' : '用全局'}
                  </button>
                ))}
              </div>
              <button
                onClick={resolveDrift}
                disabled={driftBusy}
                className={clsx('px-3 py-1 rounded-lg text-xs font-mono transition', driftBusy ? 'bg-slate-800 text-slate-500' : 'bg-amber-600 hover:bg-amber-500 text-white')}
              >
                一键解决
              </button>
            </div>
          </div>
          <ul className="space-y-1 max-h-40 overflow-y-auto">
            {drift.map((d, i) => (
              <li key={i} className="text-[11px] text-amber-200/90 font-mono">
                <span className="px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300 mr-2">
                  {DRIFT_LABEL[d.type] ?? d.type}
                </span>
                {d.carrier && <span className="text-slate-400 mr-2">[{d.carrier}]</span>}
                {d.skillId} — {d.detail}
              </li>
            ))}
          </ul>
        </div>
      )}

      {error && (
        <div className="text-center text-rose-400 text-xs py-2">{error}</div>
      )}
      {loading && <div className="text-center text-slate-500 text-xs py-4">加载中…</div>}

      {/* 技能列表 */}
      <div className="space-y-2">
        {filtered.map((it) => (
          <div
            key={it.id}
            className="p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-2"
          >
            <div className="flex items-start justify-between gap-3">
              <button
                onClick={() => openPreview(it.id)}
                className="text-left flex-1 min-w-0"
              >
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm text-slate-100">{it.name}</span>
                  <span className="text-[10px] text-slate-500 font-mono">{it.id}</span>
                  {it.category && (
                    <span className="text-[9px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                      {it.category}
                    </span>
                  )}
                  <span className={clsx('text-[9px] px-1.5 py-0.5 rounded font-mono', HEALTH_COLOR[it.mountHealth] ?? 'text-slate-500 bg-slate-800/60')}>
                    {HEALTH_LABEL[it.mountHealth] ?? it.mountHealth}
                  </span>
                  {it.security && it.security !== 'approved' && (
                    <span className={clsx('text-[9px] px-1.5 py-0.5 rounded font-mono', SECURITY_COLOR[it.security] ?? 'text-slate-500 bg-slate-800/60')}>
                      {SECURITY_LABEL[it.security] ?? it.security}
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">{it.description}</p>
              </button>
              <button
                onClick={() => toggleEnabled(it)}
                className={clsx(
                  'shrink-0 px-2.5 py-1 rounded-lg text-[11px] font-mono transition',
                  it.enabled ? 'bg-emerald-600 hover:bg-emerald-500 text-white' : 'bg-slate-800 hover:bg-slate-700 text-slate-300',
                )}
              >
                {it.enabled ? '启用中' : '已禁用'}
              </button>
            </div>

            {it.triggers.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {it.triggers.map((tr: string) => (
                  <span key={tr} className="text-[9px] px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300 font-mono">
                    {tr}
                  </span>
                ))}
              </div>
            )}

            {/* 挂载点（carrier） */}
            <div className="flex flex-wrap items-center gap-1.5 pt-1 border-t border-slate-800/60">
              <span className="text-[10px] text-slate-500 mr-1">挂载:</span>
              {CARRIERS.map((c) => {
                const on = it.mountPoints.includes(c);
                return (
                  <button
                    key={c}
                    onClick={() => toggleCarrier(it, c)}
                    className={clsx(
                      'text-[10px] px-2 py-0.5 rounded font-mono transition',
                      on ? 'bg-indigo-600 text-white' : 'bg-slate-800 text-slate-400 hover:bg-slate-700',
                    )}
                  >
                    {c}
                  </button>
                );
              })}
              {it.mountPoints.length === 0 && (
                <span className="text-[10px] text-slate-600">全量（默认）</span>
              )}
            </div>
          </div>
        ))}
        {!loading && filtered.length === 0 && (
          <div className="text-center text-slate-600 text-xs py-6">无匹配技能</div>
        )}
      </div>

      {/* 预览弹窗 */}
      {preview && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
          onClick={() => setPreview(null)}
        >
          <div
            className="w-full max-w-2xl max-h-[80vh] overflow-y-auto rounded-2xl bg-slate-900 border border-slate-700 p-5 space-y-3"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-base font-bold text-slate-100">{preview.name}</h3>
                <p className="text-[11px] text-slate-500 font-mono">{preview.path}</p>
              </div>
              <button
                onClick={() => setPreview(null)}
                className="px-3 py-1 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-300"
              >
                关闭
              </button>
            </div>

            {/* 安全 / 权限状态（内外源隔离） */}
            <div className="flex items-center justify-between gap-2 rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="text-[11px] text-slate-400">
                安全状态：
                <span className={clsx('ml-1 px-1.5 py-0.5 rounded font-mono', SECURITY_COLOR[preview.security ?? 'approved'] ?? SECURITY_COLOR.approved)}>
                  {SECURITY_LABEL[preview.security ?? 'approved'] ?? preview.security ?? '已批准'}
                </span>
                {preview.source && preview.source !== 'packs' && (
                  <span className="ml-2 text-amber-400/80">外部源（需人工批准方可注入）</span>
                )}
              </div>
              <div className="flex items-center gap-1.5">
                <button
                  onClick={() => securityAction(preview.id, 'approve')}
                  className="px-2.5 py-1 rounded-lg text-[11px] font-mono bg-emerald-600 hover:bg-emerald-500 text-white"
                >
                  批准
                </button>
                <button
                  onClick={() => securityAction(preview.id, 'quarantine')}
                  className="px-2.5 py-1 rounded-lg text-[11px] font-mono bg-rose-600 hover:bg-rose-500 text-white"
                >
                  隔离
                </button>
              </div>
            </div>

            <pre className="text-[11px] text-slate-300 whitespace-pre-wrap bg-slate-950 rounded-lg p-3 border border-slate-800 max-h-[55vh] overflow-y-auto">
{preview.content}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
