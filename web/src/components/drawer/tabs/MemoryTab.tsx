import { useCallback, useEffect, useState } from 'react';
import { useMemory } from '../../../hooks/useMemory';
import { useI18n } from '../../../store/useI18n';
import { memoryService, type LaneDisposition, type RecallOutcome } from '../../../services/memoryService';
import { breedService } from '../../../services/breedService';
import type { LaneEntryApi, RecallEventApi, RecallLedgerApi } from '../../../types/api';
import { MemoryGapPanel } from './MemoryGapPanel';

export function MemoryTab() {
  const { t } = useI18n();
  // i18n lane labels (Pending Identity lane types). Falls back to the raw type.
  const laneLabel = (type: string) => t('drawer.memory.lane.' + type);
  const { memories, loading, error } = useMemory();

  const [pending, setPending] = useState<LaneEntryApi[]>([]);
  const [truth, setTruth] = useState<LaneEntryApi[]>([]);
  const [recallEvents, setRecallEvents] = useState<RecallEventApi[]>([]);
  const [recallLedger, setRecallLedger] = useState<RecallLedgerApi>({});
  const [pullQuery, setPullQuery] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [reflectFocus, setReflectFocus] = useState('');
  const [reflection, setReflection] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [lanesError, setLanesError] = useState<string | null>(null);

  // Operator scope selector (multi-operator explicit attribution, Task #44).
  // Defaults to "" → backend uses its defaultOperator; selecting a dog scopes
  // link / sensitivity / recall-outcome writes to that acting operator.
  const [operator, setOperator] = useState('');
  const [operators, setOperators] = useState<string[]>([]);

  const refresh = useCallback(async () => {
    try {
      const [p, tr, ev, ledger] = await Promise.all([
        memoryService.getLanesPending(),
        memoryService.getLanesTruth(),
        memoryService.getRecallEvents(20),
        memoryService.getRecallLedger('7,14,30'),
      ]);
      setPending(p);
      setTruth(tr);
      setRecallEvents(ev);
      setRecallLedger(ledger);
      setLanesError(null);
    } catch (e) {
      setLanesError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    breedService.getBreeds().then((bs) => setOperators(bs.map((b) => b.id))).catch(() => {});
  }, []);

  const onDispose = async (id: string, action: LaneDisposition, content?: string) => {
    setBusy(true);
    try {
      await memoryService.disposeLane(id, action, content);
      await refresh();
    } catch (e) {
      setLanesError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

      const onMarkOutcome = async (id: string, outcome: RecallOutcome) => {
    setBusy(true);
    try {
      await memoryService.markRecallOutcomeDetailed(id, outcome, '', '', operator);
      await refresh();
    } catch (e) {
      setLanesError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const onPull = async () => {
    setBusy(true);
    try {
      await memoryService.pullRecall(pullQuery.trim());
      await refresh();
    } catch (e) {
      setLanesError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const onSearch = async () => {
    const q = searchQuery.trim();
    if (!q) return;
    setBusy(true);
    try {
      const hits = await memoryService.searchLanes(q);
      // Surface search hits as a transient pending-style list via truth+pending
      // is overkill; just refresh and let the operator scroll. We log count.
      setLanesError(hits.length === 0 ? t('drawer.memory.searchEmpty') : null);
    } catch (e) {
      setLanesError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  // LLM abstractive reflection over approved truth (P2-6, sanctioned synthesis
  // service). Output is shown, not auto-committed (human disposition required).
  const onReflect = async () => {
    setBusy(true);
    try {
      const res = await memoryService.reflectLanes({ focus: reflectFocus.trim() || undefined });
      setReflection(res.reflection || '');
      setLanesError(res.reflection ? null : t('drawer.memory.reflectEmpty'));
    } catch (e) {
      setReflection(null);
      setLanesError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const ledgerWindows = Object.keys(recallLedger).sort();
  const entryIds = [...pending, ...truth].map((e) => e.id);

  return (
    <div className="space-y-4">
      {/* Operator scope (multi-operator explicit attribution, Task #44) */}
      <div className="flex gap-1.5 items-center">
        <span className="text-[10px] text-slate-400 uppercase tracking-wider whitespace-nowrap">{t('drawer.memory.operator')}</span>
        <select
          value={operator}
          onChange={(e) => setOperator(e.target.value)}
          className="flex-1 px-2 py-1 rounded bg-slate-900 border border-slate-700 text-[11px] text-slate-200"
        >
          <option value="">— {t('drawer.memory.operatorDefault')} —</option>
          {operators.map((op) => (
            <option key={op} value={op}>{op}</option>
          ))}
        </select>
      </div>

      {/* Evidence (read-only captured memory) */}
      <section className="space-y-2">
        <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block">Shared Memory</span>
        {loading && <div className="text-center text-slate-500 text-xs py-2">{t('common.loading')}</div>}
        {error && <div className="text-center text-rose-400 text-xs py-2">{t('common.error')}: {error}</div>}
        {!loading && !error && memories.length === 0 && (
          <div className="text-center text-slate-500 text-xs py-2">{t('drawer.memory.empty')}</div>
        )}
        <div className="space-y-2">
          {memories.map((mem) => (
            <div key={mem.id} className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
              <div className="flex items-center justify-between font-mono text-[10px]">
                <span className="text-amber-400 font-bold">[{mem.type}]</span>
                <span className="text-slate-500">{mem.time}</span>
              </div>
              <p className="text-[11px] text-slate-300">{mem.fact}</p>
              <div className="text-[9px] text-slate-500 font-mono text-right">By: {mem.author}</div>
            </div>
          ))}
        </div>
      </section>

      {/* Pending candidates — awaiting human disposition (M5 提交权) */}
      <section className="space-y-2">
        <span className="font-bold text-[11px] uppercase tracking-wider text-amber-400 block">{t('drawer.memory.pending')}</span>
        {lanesError && <div className="text-center text-rose-400 text-xs py-2">{t('common.error')}: {lanesError}</div>}
        {!lanesError && pending.length === 0 && (
          <div className="text-center text-slate-500 text-xs py-2">{t('drawer.memory.pendingEmpty')}</div>
        )}
        <div className="space-y-2">
          {pending.map((e) => {
            const deferred = e.status === 'deferred';
            return (
              <div key={e.id} className={`p-2.5 rounded-xl bg-slate-950/60 border space-y-1 ${deferred ? 'border-slate-800 opacity-70' : 'border-slate-700'}`}>
                <div className="flex items-center justify-between font-mono text-[10px]">
                  <span className="text-amber-400 font-bold">[{laneLabel(e.type)}]</span>
                  <span className="text-slate-500">{new Date(e.timestamp).toLocaleDateString()}</span>
                </div>
                <p className="text-[11px] text-slate-300">{e.content}</p>
                <div className="text-[9px] text-slate-500 font-mono">{t('drawer.memory.source')}: {e.source}</div>
                {e.sensitivity ? (
                  <div className="text-[9px] text-amber-300 font-mono">{t('drawer.memory.sensitivity')}: {e.sensitivity}</div>
                ) : null}
                <div className="flex gap-1.5 pt-1 flex-wrap">
                  {!deferred && (
                    <>
                      <button
                        disabled={busy}
                        onClick={() => onDispose(e.id, 'approve')}
                        className="px-2 py-0.5 rounded bg-emerald-600/80 text-white text-[10px] disabled:opacity-50"
                      >
                        {t('drawer.memory.approve')}
                      </button>
                      <button
                        disabled={busy}
                        onClick={() => onDispose(e.id, 'reject')}
                        className="px-2 py-0.5 rounded bg-rose-600/80 text-white text-[10px] disabled:opacity-50"
                      >
                        {t('drawer.memory.reject')}
                      </button>
                      <button
                        disabled={busy}
                        onClick={() => onDispose(e.id, 'defer')}
                        className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                      >
                        {t('drawer.memory.defer')}
                      </button>
                    </>
                  )}
                  {deferred && (
                    <button
                      disabled={busy}
                      onClick={() => onDispose(e.id, 'undo')}
                      className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                    >
                      {t('drawer.memory.undo')} · {t('drawer.memory.pending')}
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* Approved truth — recalled into dog prompts (P4) */}
      <section className="space-y-2">
        <span className="font-bold text-[11px] uppercase tracking-wider text-emerald-400 block">{t('drawer.memory.truth')}</span>
        {!lanesError && truth.length === 0 && (
          <div className="text-center text-slate-500 text-xs py-2">{t('drawer.memory.truthEmpty')}</div>
        )}
        <div className="space-y-2">
          {truth.map((e) => (
            <div key={e.id} className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
              <div className="flex items-center justify-between font-mono text-[10px]">
                <span className="text-emerald-400 font-bold">[{laneLabel(e.type)}]</span>
                <span className="text-slate-500">{new Date(e.timestamp).toLocaleDateString()}</span>
              </div>
              <p className="text-[11px] text-slate-300">{e.content}</p>
              <div className="text-[9px] text-slate-500 font-mono">{t('drawer.memory.source')}: {e.source}</div>
              {e.sensitivity ? (
                <div className="text-[9px] text-amber-300 font-mono">{t('drawer.memory.sensitivity')}: {e.sensitivity}</div>
              ) : null}
              <div className="flex gap-1.5 pt-1 flex-wrap">
                <button
                  disabled={busy}
                  onClick={() => onDispose(e.id, 'retire')}
                  className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                >
                  {t('drawer.memory.retire')}
                </button>
                <button
                  disabled={busy}
                  onClick={() => onDispose(e.id, 'forget')}
                  className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                >
                  {t('drawer.memory.forget')}
                </button>
                <button
                  disabled={busy}
                  onClick={() => onDispose(e.id, 'withdraw')}
                  className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                >
                  {t('drawer.memory.withdraw')}
                </button>
                <button
                  disabled={busy}
                  onClick={() => onDispose(e.id, 'undo')}
                  className="px-2 py-0.5 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
                >
                  {t('drawer.memory.undo')}
                </button>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Recall observability — what memory was surfaced into dog prompts
          (homologous clowder RecallFeed / RecallLedger; SG's largest prior gap). */}
      <section className="space-y-2">
        <span className="font-bold text-[11px] uppercase tracking-wider text-sky-400 block">{t('drawer.memory.recall')}</span>

        {/* On-demand pull recall (clowder RecallFeed pull mode) */}
        <div className="flex gap-1.5">
          <input
            value={pullQuery}
            onChange={(ev) => setPullQuery(ev.target.value)}
            placeholder={t('drawer.memory.pullPlaceholder')}
            className="flex-1 px-2 py-1 rounded bg-slate-900 border border-slate-700 text-[11px] text-slate-200 placeholder:text-slate-600"
          />
          <button
            disabled={busy}
            onClick={onPull}
            className="px-2 py-1 rounded bg-sky-600/80 text-white text-[10px] disabled:opacity-50"
          >
            {t('drawer.memory.pull')}
          </button>
        </div>

        {/* Full-text search (FTS5, P1-5) */}
        <div className="flex gap-1.5">
          <input
            value={searchQuery}
            onChange={(ev) => setSearchQuery(ev.target.value)}
            placeholder={t('drawer.memory.searchPlaceholder')}
            className="flex-1 px-2 py-1 rounded bg-slate-900 border border-slate-700 text-[11px] text-slate-200 placeholder:text-slate-600"
          />
          <button
            disabled={busy}
            onClick={onSearch}
            className="px-2 py-1 rounded bg-slate-700 text-white text-[10px] disabled:opacity-50"
          >
            {t('drawer.memory.search')}
          </button>
        </div>

        {lanesError && <div className="text-center text-rose-400 text-xs py-2">{t('common.error')}: {lanesError}</div>}
        {!lanesError && recallEvents.length === 0 && (
          <div className="text-center text-slate-500 text-xs py-2">{t('drawer.memory.recallEmpty')}</div>
        )}
        <div className="space-y-2">
          {recallEvents.map((ev) => (
            <div key={ev.id} className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
              <div className="flex items-center justify-between font-mono text-[10px]">
                <span className="text-sky-400 font-bold">{ev.kind} · {ev.trigger}</span>
                <span className="text-slate-500">{new Date(ev.timestamp).toLocaleString()}</span>
              </div>
              <div className="text-[9px] text-slate-500 font-mono">
                {t('drawer.memory.recallEntries')}: {ev.count} · {t('drawer.memory.recallTrigger')}: {ev.trigger}
              </div>
              {/* Consumption verification (P0-1): operator marks used/ignored. */}
              {ev.outcome ? (
                <div className="text-[9px] font-mono">
                  {t('drawer.memory.outcome')}:{' '}
                  <span className={ev.outcome === 'used' ? 'text-emerald-400' : 'text-rose-400'}>
                    {ev.outcome === 'used' ? t('drawer.memory.outcomeUsed') : t('drawer.memory.outcomeIgnored')}
                  </span>
                </div>
              ) : (
                <div className="flex gap-1.5 pt-1">
                  <button
                    disabled={busy}
                    onClick={() => onMarkOutcome(ev.id, 'used')}
                    className="px-2 py-0.5 rounded bg-emerald-600/80 text-white text-[10px] disabled:opacity-50"
                  >
                    {t('drawer.memory.outcomeUsed')}
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => onMarkOutcome(ev.id, 'ignored')}
                    className="px-2 py-0.5 rounded bg-rose-600/80 text-white text-[10px] disabled:opacity-50"
                  >
                    {t('drawer.memory.outcomeIgnored')}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
        {laneKeysPresent(recallLedger) && (
          <div className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
            <div className="font-bold text-[10px] text-slate-400 block">{t('drawer.memory.recallLedger')}</div>
            <div className="flex gap-3 flex-wrap">
              {ledgerWindows.map((w) => {
                const s = recallLedger[w];
                return (
                  <span key={w} className="text-[10px] text-slate-300 font-mono">
                    {w}: <span className="text-sky-400 font-bold">{s.total}</span>{t('drawer.memory.recallCount')}
                    {' '}({t('drawer.memory.ledgerUsed')} {s.used} / {t('drawer.memory.ledgerIgnored')} {s.ignored} / {t('drawer.memory.ledgerRate')} {(s.rate * 100).toFixed(0)}%)
                  </span>
                );
              })}
            </div>
            {/* Three-axis recall semantics (P1): beneficial / unmet / attention */}
            <div className="flex gap-2 flex-wrap pt-1">
              {ledgerWindows.map((w) => {
                const s = recallLedger[w];
                if (!s || (s.beneficial === 0 && s.unmet === 0 && s.attention === 0)) return null;
                return (
                  <span key={w} className="text-[10px] font-mono">
                    {w}:{' '}
                    <span className="text-emerald-400">{t('drawer.memory.axisBeneficial')} {s.beneficial}</span> /{' '}
                    <span className="text-amber-400">{t('drawer.memory.axisUnmet')} {s.unmet}</span> /{' '}
                    <span className="text-rose-400">{t('drawer.memory.axisAttention')} {s.attention}</span>
                  </span>
                );
              })}
            </div>
            <div className="flex gap-2 flex-wrap pt-0.5 text-[9px] text-slate-500 font-mono">
              {Object.entries(recallLedger).flatMap(([w, s]) =>
                Object.entries(s.maturity || {}).map(([m, c]) => (
                  <span key={w + m}>{w} {t('drawer.memory.maturity' + m.charAt(0).toUpperCase() + m.slice(1))}: {c}</span>
                )),
              )}
            </div>
          </div>
        )}
      </section>

      {/* LLM abstractive reflection over approved truth (P2-6, sanctioned
          synthesis service). Output is shown, never auto-committed. */}
      <section className="space-y-2">
        <span className="font-bold text-[11px] uppercase tracking-wider text-fuchsia-400 block">{t('drawer.memory.reflect')}</span>
        <div className="flex gap-1.5">
          <input
            value={reflectFocus}
            onChange={(ev) => setReflectFocus(ev.target.value)}
            placeholder={t('drawer.memory.reflectPlaceholder')}
            className="flex-1 px-2 py-1 rounded bg-slate-900 border border-slate-700 text-[11px] text-slate-200 placeholder:text-slate-600"
          />
          <button
            disabled={busy}
            onClick={onReflect}
            className="px-2 py-1 rounded bg-fuchsia-600/80 text-white text-[10px] disabled:opacity-50"
          >
            {t('drawer.memory.reflect')}
          </button>
        </div>
        {reflection !== null && (
          <div className="p-2.5 rounded-xl bg-slate-950/60 border border-fuchsia-800/60 space-y-1">
            <div className="font-bold text-[10px] text-fuchsia-300 block">{t('drawer.memory.reflectResult')}</div>
            <p className="text-[11px] text-slate-300 whitespace-pre-wrap">{reflection || t('drawer.memory.reflectEmpty')}</p>
          </div>
        )}
      </section>

      {/* GAP capabilities (homologous clowder edges/markers/sensitivity/vec0) */}
      <MemoryGapPanel entryIds={entryIds} onChanged={refresh} operator={operator} />
    </div>
  );
}

function laneKeysPresent(ledger: RecallLedgerApi): boolean {
  return Object.keys(ledger).length > 0;
}
