import { useCallback, useEffect, useState } from 'react';
import {
  getDossierOverview,
  listObservations,
  addObservation,
  listProposals,
  approveProposal,
  rejectProposal,
} from '../../services/dossierService';
import type { DossierOverview, DistillationProposal, DossierObservation } from '../../types/dossier';

function StatusBadge({ status }: { status: DistillationProposal['status'] }) {
  const map: Record<string, string> = {
    pending: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
    approved: 'bg-sky-500/15 text-sky-300 border-sky-500/30',
    rejected: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
    applied: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
  };
  const label: Record<string, string> = { pending: '待审批', approved: '已批准', rejected: '已否决', applied: '已应用' };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs border ${map[status] ?? 'bg-slate-500/15 text-slate-300 border-slate-500/30'}`}>
      {label[status] ?? status}
    </span>
  );
}

function ProvenanceBadge({ version, date }: { version?: string; date?: string }) {
  if (!version && !date) return null;
  return (
    <span className="text-xs text-slate-500">
      provenance v{version ?? '?'} · {date ?? '?'}
    </span>
  );
}

export default function DogDossierPanel() {
  const [overview, setOverview] = useState<DossierOverview | null>(null);
  const [observations, setObservations] = useState<Record<string, DossierObservation[]>>({});
  const [proposals, setProposals] = useState<DistillationProposal[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [obsDogId, setObsDogId] = useState('');
  const [obsContent, setObsContent] = useState('');
  const [busy, setBusy] = useState('');

  const dogIds = overview?.modelGroups.flatMap(g => g.dogs.map(d => d.dogId)) ?? [];

  const refresh = useCallback(async () => {
    try {
      setError('');
      const [ov, obs, props] = await Promise.all([
        getDossierOverview(),
        listObservations() as Promise<Record<string, DossierObservation[]>>,
        listProposals(),
      ]);
      setOverview(ov);
      setObservations(obs);
      setProposals(props);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  const submitObservation = async () => {
    if (!obsDogId || !obsContent.trim()) return;
    setBusy('obs');
    try {
      await addObservation(obsDogId, obsContent.trim());
      setObsContent('');
      await refresh();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy('');
    }
  };

  const act = async (id: string, action: 'approve' | 'reject') => {
    setBusy(`${action}:${id}`);
    try {
      if (action === 'approve') await approveProposal(id);
      else await rejectProposal(id, 'operator 否决');
      await refresh();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy('');
    }
  };

  if (loading) {
    return <div className="text-slate-400">加载能力画像档案…</div>;
  }

  return (
    <div className="space-y-8 text-slate-200">
      {error && (
        <div className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-4 py-2 text-sm text-rose-300">
          {error}
        </div>
      )}

      {/* 概览 */}
      {overview && (
        <section className="rounded-xl border border-slate-700/60 bg-slate-900/60 p-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold">档案覆盖</h3>
            {!overview.meta.dossierAvailable && (
              <span className="text-xs text-amber-300">dog-dossier.md 未找到（社区回退模式）</span>
            )}
          </div>
          <div className="mt-2 flex gap-6 text-sm text-slate-400">
            <span>狗狗 <b className="text-slate-200">{overview.meta.totalDogs}</b></span>
            <span>模型 <b className="text-slate-200">{overview.meta.totalModels}</b></span>
            <span>覆盖率 <b className="text-slate-200">{Math.round(overview.meta.dossierCoverage * 100)}%</b></span>
          </div>
          <p className="mt-2 text-xs text-slate-500">
            画像描述模型的认知能力（dogId 是索引）。更新走蒸馏提案审批流：犬提案 → operator 审批 → 目标犬应用。禁止性格打分。
          </p>
        </section>
      )}

      {/* 模型分组画像 */}
      {overview?.modelGroups.map(group => (
        <section key={group.model}>
          <h3 className="mb-2 text-sm font-semibold text-slate-300">
            {group.model} <span className="text-slate-500 font-normal">({group.dogs.length})</span>
          </h3>
          <div className="grid gap-3 md:grid-cols-2">
            {group.dogs.map(dog => {
              const p = dog.dossier;
              return (
                <div key={dog.dogId} className="rounded-xl border border-slate-700/60 bg-slate-900/60 p-4">
                  <div className="flex items-baseline justify-between gap-2">
                    <div>
                      <span className="font-medium">{dog.displayName}</span>
                      <span className="ml-2 text-xs text-slate-500">{dog.dogId}</span>
                      {dog.channel && <span className="ml-1 text-xs text-slate-600">· {dog.channel}</span>}
                    </div>
                    <ProvenanceBadge version={p?.provenance?.version} date={p?.provenance?.date} />
                  </div>
                  {p ? (
                    <div className="mt-2 space-y-1.5 text-sm">
                      {p.oneLiner && <p className="text-slate-300">{p.oneLiner}</p>}
                      {p.l0RoutingNote && (
                        <p className="text-xs text-slate-400">
                          <span className="text-slate-500">路由边界：</span>{p.l0RoutingNote}
                        </p>
                      )}
                      {!!p.routingSignals?.peakCapabilities?.length && (
                        <p className="text-xs text-emerald-300/80">▲ {p.routingSignals.peakCapabilities.join('；')}</p>
                      )}
                      {!!p.routingSignals?.antiSignals?.length && (
                        <p className="text-xs text-rose-300/70">▽ {p.routingSignals.antiSignals.join('；')}</p>
                      )}
                      <p className="text-xs text-slate-600">名册摘要：{p.l0RosterSummary ?? '—'}</p>
                    </div>
                  ) : (
                    <p className="mt-2 text-xs text-amber-300/80">无档案条目（回退 config 人设）</p>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      ))}

      {/* 观察暂存层 */}
      <section className="rounded-xl border border-slate-700/60 bg-slate-900/60 p-4">
        <h3 className="text-sm font-semibold">观察暂存层（operator 体感）</h3>
        <p className="mt-1 text-xs text-slate-500">
          观察只进暂存层，作为蒸馏提案的证据被引用；不直接覆盖总结层。
        </p>
        <div className="mt-3 flex flex-wrap gap-2">
          <select
            value={obsDogId}
            onChange={e => setObsDogId(e.target.value)}
            className="rounded-lg border border-slate-700 bg-slate-950 px-2 py-1.5 text-sm"
          >
            <option value="">选择狗狗…</option>
            {dogIds.map(id => <option key={id} value={id}>{id}</option>)}
          </select>
          <input
            value={obsContent}
            onChange={e => setObsContent(e.target.value)}
            placeholder="一句话观察（如：拆解又快又准 / review 时越界改了架构）"
            className="min-w-64 flex-1 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-sm"
            onKeyDown={e => { if (e.key === 'Enter') void submitObservation(); }}
          />
          <button
            onClick={() => void submitObservation()}
            disabled={!obsDogId || !obsContent.trim() || busy === 'obs'}
            className="rounded-lg bg-indigo-600 px-3 py-1.5 text-sm text-white disabled:opacity-40"
          >
            {busy === 'obs' ? '提交中…' : '添加观察'}
          </button>
        </div>
        {Object.keys(observations).length > 0 && (
          <ul className="mt-3 space-y-1 text-xs text-slate-400">
            {Object.entries(observations).flatMap(([dogId, obs]) =>
              obs.slice(0, 3).map(o => (
                <li key={o.id}>
                  <span className="text-slate-500">{dogId}</span> · {o.content}
                  <span className="ml-1 text-slate-600">（{o.provenance.author} {o.provenance.date}）</span>
                </li>
              )),
            )}
          </ul>
        )}
      </section>

      {/* 蒸馏提案 */}
      <section className="rounded-xl border border-slate-700/60 bg-slate-900/60 p-4">
        <h3 className="text-sm font-semibold">蒸馏提案</h3>
        <p className="mt-1 text-xs text-slate-500">
          待审批列表（默认拉取 pending）。批准后由目标犬调用 execute-apply 写入档案并 commit（不 push）。
        </p>
        {proposals.length === 0 ? (
          <p className="mt-3 text-sm text-slate-500">暂无待审批提案。</p>
        ) : (
          <ul className="mt-3 space-y-3">
            {proposals.map(p => (
              <li key={p.proposalId} className="rounded-lg border border-slate-700/50 bg-slate-950/60 p-3 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <StatusBadge status={p.status} />
                    <span className="font-medium">{p.targetDogId}</span>
                    <span className="text-xs text-slate-500">{p.targetFields.join(', ')}</span>
                  </div>
                  {p.status === 'pending' && (
                    <div className="flex gap-2">
                      <button
                        onClick={() => void act(p.proposalId, 'approve')}
                        disabled={busy === `approve:${p.proposalId}`}
                        className="rounded-lg bg-emerald-600/80 px-2.5 py-1 text-xs text-white disabled:opacity-40"
                      >
                        批准
                      </button>
                      <button
                        onClick={() => void act(p.proposalId, 'reject')}
                        disabled={busy === `reject:${p.proposalId}`}
                        className="rounded-lg bg-rose-600/80 px-2.5 py-1 text-xs text-white disabled:opacity-40"
                      >
                        否决
                      </button>
                    </div>
                  )}
                </div>
                <p className="mt-1.5 text-slate-300">{p.rationale}</p>
                <div className="mt-1.5 grid gap-1 text-xs md:grid-cols-2">
                  <pre className="overflow-x-auto rounded bg-rose-500/5 p-2 text-rose-200/80 whitespace-pre-wrap">{p.beforeSnapshot}</pre>
                  <pre className="overflow-x-auto rounded bg-emerald-500/5 p-2 text-emerald-200/80 whitespace-pre-wrap">{p.afterDraft}</pre>
                </div>
                <p className="mt-1.5 text-xs text-slate-600">
                  证据：{p.evidenceRefs.map(r => `${r.type}:${r.id}`).join('，')} · 提案者 {p.createdBy}
                  {p.appliedCommitSha && ` · commit ${p.appliedCommitSha.slice(0, 8)}`}
                </p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
