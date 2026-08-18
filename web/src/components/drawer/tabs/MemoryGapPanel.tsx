import { useCallback, useEffect, useState } from 'react';
import { useI18n } from '../../../store/useI18n';
import { memoryService } from '../../../services/memoryService';
import {
  LANE_RELATIONS,
  SENSITIVITY_LEVELS,
  type LaneEdgeApi,
  type LaneMarkerApi,
  type LaneEntryApi,
  type SensitivityLevel,
  type LifecycleTraceApi,
} from '../../../types/api';

interface Props {
  entryIds: string[];
  onChanged: () => void;
  operator?: string;
}

const SENS_KEY: Record<SensitivityLevel, string> = {
  public: 'drawer.memory.sensitivityPublic',
  internal: 'drawer.memory.sensitivityInternal',
  private: 'drawer.memory.sensitivityPrivate',
  restricted: 'drawer.memory.sensitivityRestricted',
};

const REL_COLOR: Record<string, string> = {
  evolved_from: '#f59e0b',
  blocked_by: '#ef4444',
  supersedes: '#22c55e',
  invalidates: '#ef4444',
  related: '#38bdf8',
  related_to: '#38bdf8',
  promoted_from: '#a78bfa',
  wikilink: '#94a3b8',
  doc_link: '#94a3b8',
  feature_ref: '#94a3b8',
};

const short = (id: string) => id.slice(0, 8);
const DIM = new Set(['private', 'restricted']);

// Frontend surface for the Shared-Memory GAPs (homologous clowder edges,
// markers, 4-tier CollectionSensitivity, vec0 semantic recall, and three-axis
// recall semantics). All write actions are operator-initiated; nothing
// auto-commits.
export function MemoryGapPanel({ entryIds, onChanged, operator = '' }: Props) {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Relationship graph explorer (Gap1).
  const [graphSource, setGraphSource] = useState('');
  const [edges, setEdges] = useState<LaneEdgeApi[]>([]);
  const [markers, setMarkers] = useState<LaneMarkerApi[]>([]);
  const [linkTarget, setLinkTarget] = useState('');
  const [linkRelation, setLinkRelation] = useState(LANE_RELATIONS[0]);
  const [linkSens, setLinkSens] = useState<SensitivityLevel | ''>('');
  const [markType, setMarkType] = useState('decision');
  const [markContent, setMarkContent] = useState('');

  // Sensitivity setter (Gap2) + visibility-widening guardrail (Task #33).
  const [sensId, setSensId] = useState('');
  const [sensLevel, setSensLevel] = useState<SensitivityLevel>('internal');
  const [widen, setWiden] = useState<{ current: string; requested: string } | null>(null);

  // Semantic recall (Gap3).
  const [semQuery, setSemQuery] = useState('');
  const [semResults, setSemResults] = useState<LaneEntryApi[]>([]);

  // Lifecycle trace (P1/Task #39).
  const [lifecycle, setLifecycle] = useState<LifecycleTraceApi[]>([]);

  const run = useCallback(async (fn: () => Promise<void>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  const loadGraph = useCallback(async (id: string) => {
    const g = await memoryService.getGraph(id);
    setEdges(g.edges);
    setMarkers(g.markers);
  }, []);

  // Auto-load lifecycle when the panel mounts.
  useEffect(() => {
    memoryService.getLifecycle(20).then(setLifecycle).catch(() => {});
  }, [onChanged]);

  const onGetGraph = () =>
    run(async () => {
      if (!graphSource) return;
      await loadGraph(graphSource);
    });

  const onLink = () =>
    run(async () => {
      if (!graphSource || !linkTarget) return;
      await memoryService.linkEntries(graphSource, linkTarget, linkRelation, linkSens, 'manual', operator);
      await loadGraph(graphSource);
      setLinkTarget('');
      onChanged();
    });

  const onMark = () =>
    run(async () => {
      if (!graphSource) return;
      await memoryService.markEntry(graphSource, markType, markContent);
      await loadGraph(graphSource);
      setMarkContent('');
      onChanged();
    });

  const onSetSens = () =>
    run(async () => {
      if (!sensId) return;
      const res = await memoryService.setSensitivity(sensId, sensLevel, false, operator);
      if (res.confirm_field) {
        // Backend rejected a visibility widening — surface confirm UI (#33).
        setWiden({ current: res.current ?? '', requested: res.requested ?? sensLevel });
        setErr(t('drawer.memory.widenRejected'));
        return;
      }
      setWiden(null);
      onChanged();
    });

  const onConfirmWiden = () =>
    run(async () => {
      if (!sensId) return;
      await memoryService.setSensitivity(sensId, sensLevel, true, operator);
      setWiden(null);
      onChanged();
    });

  const onSemantic = () =>
    run(async () => {
      const q = semQuery.trim();
      if (!q) return;
      setSemResults([]);
      try {
        const r = await memoryService.semanticSearch(q, 10);
        setSemResults(r);
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        setErr(/501/.test(msg) ? t('drawer.memory.semanticEmpty') : msg);
      }
    });

  const onReindex = () =>
    run(async () => {
      await memoryService.reindexVectors();
      onChanged();
    });

  const inputCls =
    'px-2 py-1 rounded bg-slate-900 border border-slate-700 text-[11px] text-slate-200 placeholder:text-slate-600';
  const btnCls = 'px-2 py-1 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50';
  const primaryBtn = 'px-2 py-1 rounded bg-sky-600/80 text-white text-[10px] disabled:opacity-50';

  // SVG relationship graph: center node + target nodes placed radially.
  const targets = edges.map((e) => e.to_id);
  const n = targets.length;

  return (
    <section className="space-y-3">
      <span className="font-bold text-[11px] uppercase tracking-wider text-indigo-400 block">
        {t('drawer.memory.graph')}
      </span>

      {/* Relationship graph: pick source, link/mark, inspect edges+markers (Gap1) */}
      <div className="space-y-2 p-2.5 rounded-xl bg-slate-950/60 border border-slate-800">
        <div className="flex gap-1.5 items-center">
          <select value={graphSource} onChange={(e) => setGraphSource(e.target.value)} className={inputCls + ' flex-1'}>
            <option value="">— {t('drawer.memory.source')} —</option>
            {entryIds.map((id) => (
              <option key={id} value={id}>{short(id)}</option>
            ))}
          </select>
          <button disabled={busy || !graphSource} onClick={onGetGraph} className={btnCls}>{t('drawer.memory.graph')}</button>
        </div>

        <div className="flex gap-1.5 items-center flex-wrap">
          <select value={linkTarget} onChange={(e) => setLinkTarget(e.target.value)} className={inputCls + ' flex-1'}>
            <option value="">— {t('drawer.memory.linkTo')} —</option>
            {entryIds.filter((id) => id !== graphSource).map((id) => (
              <option key={id} value={id}>{short(id)}</option>
            ))}
          </select>
          <select value={linkRelation} onChange={(e) => setLinkRelation(e.target.value)} className={inputCls}>
            {LANE_RELATIONS.map((r) => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
          <select value={linkSens} onChange={(e) => setLinkSens(e.target.value as SensitivityLevel)} className={inputCls}>
            <option value="">— {t('drawer.memory.edgeSensitivity')} —</option>
            {SENSITIVITY_LEVELS.map((lvl) => (
              <option key={lvl} value={lvl}>{t(SENS_KEY[lvl])}</option>
            ))}
          </select>
          <button disabled={busy || !graphSource || !linkTarget} onClick={onLink} className={primaryBtn}>{t('drawer.memory.link')}</button>
        </div>

        <div className="flex gap-1.5 items-center flex-wrap">
          <input
            value={markType}
            onChange={(e) => setMarkType(e.target.value)}
            placeholder={t('drawer.memory.markType')}
            className={inputCls + ' w-28'}
          />
          <input
            value={markContent}
            onChange={(e) => setMarkContent(e.target.value)}
            placeholder={t('drawer.memory.markPlaceholder')}
            className={inputCls + ' flex-1'}
          />
          <button disabled={busy || !graphSource} onClick={onMark} className={btnCls}>{t('drawer.memory.mark')}</button>
        </div>

        {/* SVG relationship graph (Task #38, homologous clowder CollectionGraph) */}
        {n > 0 && (
          <svg viewBox="0 0 320 200" className="w-full h-40 bg-slate-900/40 rounded">
            <line x1={160} y1={100} x2={160} y2={100} />
            {edges.map((ed, i) => {
              const ang = (Math.PI * 2 * i) / n - Math.PI / 2;
              const x = 160 + Math.cos(ang) * 90;
              const y = 100 + Math.sin(ang) * 70;
              const dim = DIM.has(ed.edge_sensitivity || '');
              return (
                <g key={ed.id} opacity={dim ? 0.4 : 1}>
                  <line x1={160} y1={100} x2={x} y2={y} stroke={REL_COLOR[ed.relation] || '#94a3b8'} strokeWidth={1.5} strokeDasharray={dim ? '3 3' : undefined} />
                  <circle cx={x} cy={y} r={14} fill="#0f172a" stroke={REL_COLOR[ed.relation] || '#94a3b8'} />
                  <text x={x} y={y + 3} textAnchor="middle" fontSize={7} fill="#cbd5e1">{short(ed.to_id)}</text>
                </g>
              );
            })}
            <circle cx={160} cy={100} r={16} fill="#1e293b" stroke="#38bdf8" />
            <text x={160} y={103} textAnchor="middle" fontSize={7} fill="#e2e8f0">{short(graphSource)}</text>
          </svg>
        )}

        {(edges.length > 0 || markers.length > 0) && (
          <div className="space-y-1.5 pt-1">
            {edges.length > 0 && (
              <div className="text-[10px] text-slate-400 font-bold">{t('drawer.memory.edges')}</div>
            )}
            {edges.map((ed) => (
              <div key={ed.id} className="text-[10px] text-slate-300 font-mono">
                <span style={{ color: REL_COLOR[ed.relation] || '#94a3b8' }}>{ed.relation}</span> → {short(ed.to_id)}
                {ed.edge_sensitivity ? ` (${ed.edge_sensitivity})` : ''}
              </div>
            ))}
            {markers.length > 0 && (
              <div className="text-[10px] text-slate-400 font-bold pt-1">{t('drawer.memory.markers')}</div>
            )}
            {markers.map((mk) => (
              <div key={mk.id} className="text-[10px] text-slate-300 font-mono">
                <span className="text-fuchsia-400">[{mk.marker_type}]</span> {mk.content || '—'} <span className="text-slate-500">({mk.status})</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 4-tier sensitivity setter (Gap2) + widening guardrail (Task #33) */}
      <div className="flex gap-1.5 items-center">
        <select value={sensId} onChange={(e) => setSensId(e.target.value)} className={inputCls + ' flex-1'}>
          <option value="">— {t('drawer.memory.source')} —</option>
          {entryIds.map((id) => (
            <option key={id} value={id}>{short(id)}</option>
          ))}
        </select>
        <select value={sensLevel} onChange={(e) => setSensLevel(e.target.value as SensitivityLevel)} className={inputCls}>
          {SENSITIVITY_LEVELS.map((lvl) => (
            <option key={lvl} value={lvl}>{t(SENS_KEY[lvl])}</option>
          ))}
        </select>
        <button disabled={busy || !sensId} onClick={onSetSens} className={btnCls}>{t('drawer.memory.setSensitivity')}</button>
      </div>
      {widen && (
        <div className="p-2 rounded bg-amber-900/30 border border-amber-700/60 text-[10px] text-amber-200 space-y-1">
          <div>{t('drawer.memory.widenConfirm').replace('{current}', widen.current).replace('{requested}', widen.requested)}</div>
          <button onClick={onConfirmWiden} className={primaryBtn}>{t('drawer.memory.widenConfirmBtn')}</button>
        </div>
      )}

      {/* Semantic recall + reindex (Gap3) */}
      <div className="space-y-2">
        <div className="flex gap-1.5 items-center">
          <input
            value={semQuery}
            onChange={(e) => setSemQuery(e.target.value)}
            placeholder={t('drawer.memory.semanticPlaceholder')}
            className={inputCls + ' flex-1'}
          />
          <button disabled={busy} onClick={onSemantic} className={primaryBtn}>{t('drawer.memory.semantic')}</button>
          <button disabled={busy} onClick={onReindex} className={btnCls}>{t('drawer.memory.reindex')}</button>
        </div>
        {semResults.length > 0 && (
          <div className="space-y-2">
            {semResults.map((r) => (
              <div key={r.id} className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
                <div className="font-mono text-[10px] text-emerald-400 font-bold">[{r.type}]</div>
                <p className="text-[11px] text-slate-300">{r.content}</p>
              </div>
            ))}
          </div>
        )}
        {semResults.length === 0 && !busy && (
          <div className="text-center text-slate-500 text-[10px] py-1">{t('drawer.memory.semanticEmpty')}</div>
        )}
      </div>

      {/* Lifecycle trace (P1 / Task #39) */}
      {lifecycle.length > 0 && (
        <div className="space-y-1">
          <div className="text-[10px] text-slate-400 font-bold">{t('drawer.memory.lifecycle')}</div>
          {lifecycle.slice(0, 8).map((l, i) => (
            <div key={i} className="text-[10px] text-slate-300 font-mono flex justify-between">
              <span>
                {l.axis === 'creation' ? t('drawer.memory.lifecycleCreation') : l.axis === 'consumption' ? t('drawer.memory.lifecycleConsumption') : t('drawer.memory.lifecycleCorrection')}
                {' '}{short(l.entry_id)}
              </span>
              <span className="text-slate-500">{l.detail}</span>
            </div>
          ))}
        </div>
      )}

      {err && <div className="text-center text-rose-400 text-xs py-1">{t('common.error')}: {err}</div>}
    </section>
  );
}
