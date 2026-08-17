import { useCallback, useEffect, useRef, useState } from 'react';
import { useI18n } from '../../store/useI18n';
import {
  listPeople,
  getPerson,
  listCandidates,
  getCandidate,
  approveCandidate,
  rejectCandidate,
  rejectDrafts,
  notNowCandidate,
  withdrawCandidate,
  undoDecision,
  correctClaim,
  amendEvent,
  retireClaim,
  redactItem,
  forgetPerson,
  listDeferred,
  claimDeferred,
  withdrawReceipt,
  forgetReceipt,
  proposeCandidate,
  listOperators,
  setActiveOperator,
  getActiveOperator,
  type PersonDetail,
} from '../../services/peopleMemoryService';
import type {
  PersonSummary,
  CaptureCandidate,
  DeferredPersonMemoryReceipt,
  PersonClaimVersion,
  InteractionEvent,
  PersonClaimPayload,
  CandidateClaimDraft,
} from '../../types/peopleMemory';

type Tab = 'people' | 'candidates' | 'deferred';

function claimText(c: { payload: { kind: string; predicate?: string; statement?: string; value?: unknown } }): string {
  if (c.payload.kind === 'reported_fact') return `${c.payload.predicate ?? ''} = ${JSON.stringify(c.payload.value ?? '')}`;
  if (c.payload.kind === 'user_assessment') return c.payload.statement ?? '';
  return c.payload.kind;
}

function temporalLabel(t?: { kind: string; value?: string; raw?: string; qualifier?: string }): string {
  if (!t) return '';
  if (t.kind === 'exact') return t.value ?? '';
  if (t.kind === 'approximate') return `${t.qualifier ?? ''} ${t.raw ?? ''}`.trim();
  return t.raw ?? '';
}

// Multi-draft capture model: a single candidate can carry several claim cards
// plus at most one relationship card and one interaction card. This mirrors
// the profile-update-actions — many drafts presented together in one
// proposal block, each independently approvable.
interface ClaimDraftForm {
  claimKind: 'reported_fact' | 'user_assessment';
  predicate: string;
  value: string;
  statement: string;
  excerpt: string;
}

function emptyClaimDraft(): ClaimDraftForm {
  return { claimKind: 'reported_fact', predicate: '', value: '', statement: '', excerpt: '' };
}

// PeopleMemoryContent — F276 People & Relationship Memory panel (homologous).
// Faithful to the backend contract: people / candidates (approval) / deferred
// receipts in one owner-private surface; no reasoning runs here — only store &
// project what the operator or a CLI dog submitted. Multi-operator scoped via
// the X-Operator-Id header (operator switcher at top).
export function PeopleMemoryContent() {
  const { t } = useI18n();
  const [tab, setTab] = useState<Tab>('people');
  const [people, setPeople] = useState<PersonSummary[]>([]);
  const [candidates, setCandidates] = useState<CaptureCandidate[]>([]);
  const [deferred, setDeferred] = useState<DeferredPersonMemoryReceipt[]>([]);
  const [detail, setDetail] = useState<PersonDetail | null>(null);
  const [cand, setCand] = useState<CaptureCandidate | null>(null);
  const [selectedPerson, setSelectedPerson] = useState<string | null>(null);
  const [selectedCand, setSelectedCand] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  // Multi-operator switcher state.
  const [operators, setOperators] = useState<string[]>([]);
  const [operatorInput, setOperatorInput] = useState(getActiveOperator() ?? '');
  // The operator currently applied (drives the live-sync SSE subscription).
  const [appliedOperator, setAppliedOperator] = useState(getActiveOperator() ?? '');
  // Live-sync connection indicator: 'connecting' | 'open' | 'error'.
  const [liveState, setLiveState] = useState<'connecting' | 'open' | 'error'>('connecting');
  const [showCapture, setShowCapture] = useState(false);
  const [busy, setBusy] = useState(false);
  // Multi-draft capture: N claim cards + at most one relationship + one interaction.
  const [capturePerson, setCapturePerson] = useState({ displayName: '', aliases: '', targetPersonID: '' });
  const [claimDrafts, setClaimDrafts] = useState<ClaimDraftForm[]>([emptyClaimDraft()]);
  const [relationshipDraft, setRelationshipDraft] = useState<{ status: 'current' | 'former' | 'unknown' } | null>(null);
  const [interactionDraft, setInteractionDraft] = useState<{ eventKind: string; headline: string; occurredAt: string; importance: string } | null>(null);

  const loadPeople = useCallback(async () => {
    setLoading(true);
    try {
      setPeople(await listPeople());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadCandidates = useCallback(async () => {
    setLoading(true);
    try {
      setCandidates(await listCandidates());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadDeferred = useCallback(async () => {
    setLoading(true);
    try {
      setDeferred(await listDeferred());
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadOperators = useCallback(async () => {
    try {
      setOperators(await listOperators());
    } catch {
      /* operator discovery is best-effort */
    }
  }, []);

  useEffect(() => {
    if (tab === 'people') loadPeople();
    else if (tab === 'candidates') loadCandidates();
    else loadDeferred();
  }, [tab, loadPeople, loadCandidates, loadDeferred]);

  useEffect(() => {
    loadOperators();
  }, [loadOperators]);

  // Live cross-tab sync: subscribe to the people-memory SSE stream for the
  // applied operator and refresh the current view whenever another tab changes
  // anything (approve/reject/fold/edit). The subscription re-opens when the
  // operator scope changes; reloadRef keeps the latest reload fn without
  // re-subscribing on every selection.
  const reloadRef = useRef<() => void>(() => {});
  useEffect(() => {
    setLiveState('connecting');
    const es = new EventSource(
      `/api/people-memory/events?operator=${encodeURIComponent(appliedOperator)}`,
    );
    es.onopen = () => setLiveState('open');
    es.onmessage = () => reloadRef.current();
    es.onerror = () => setLiveState('error'); // EventSource auto-reconnects
    return () => es.close();
  }, [appliedOperator]);

  const openPerson = useCallback(async (id: string) => {
    setLoading(true);
    setError('');
    try {
      const d = await getPerson(id);
      setDetail(d);
      setSelectedPerson(id);
      setCand(null);
      setSelectedCand(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  const openCandidate = useCallback(async (id: string) => {
    setLoading(true);
    try {
      setCand(await getCandidate(id));
      setSelectedCand(id);
      setDetail(null);
      setSelectedPerson(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  const reloadCurrent = useCallback(() => {
    if (selectedPerson) openPerson(selectedPerson);
    if (selectedCand) openCandidate(selectedCand);
    if (tab === 'candidates') loadCandidates();
    if (tab === 'deferred') loadDeferred();
    if (tab === 'people') loadPeople();
  }, [selectedPerson, selectedCand, tab, openPerson, openCandidate, loadCandidates, loadDeferred, loadPeople]);
  reloadRef.current = reloadCurrent;

  const applyOperator = useCallback(
    (op: string) => {
      const trimmed = op.trim();
      setActiveOperator(trimmed || null);
      setOperatorInput(getActiveOperator() ?? '');
      setAppliedOperator(trimmed);
      setSelectedPerson(null);
      setSelectedCand(null);
      setDetail(null);
      setCand(null);
      if (tab === 'people') loadPeople();
      else if (tab === 'candidates') loadCandidates();
      else loadDeferred();
      loadOperators();
    },
    [tab, loadPeople, loadCandidates, loadDeferred, loadOperators],
  );

  const resetCapture = useCallback(() => {
    setCapturePerson({ displayName: '', aliases: '', targetPersonID: '' });
    setClaimDrafts([emptyClaimDraft()]);
    setRelationshipDraft(null);
    setInteractionDraft(null);
  }, []);

  const toggleCapture = useCallback(
    (open: boolean) => {
      setShowCapture(open);
      if (open) void loadPeople(); // populate target-person select
    },
    [loadPeople],
  );

  const submitCapture = useCallback(async () => {
    const isNew = !capturePerson.targetPersonID;
    if (isNew && !capturePerson.displayName.trim()) {
      setError('显示名必填');
      return;
    }
    if (claimDrafts.length === 0 && !relationshipDraft && !interactionDraft) {
      setError('至少添加一个草稿卡片');
      return;
    }
    const claimArr: CandidateClaimDraft[] = claimDrafts.map((c) => {
      const payload: PersonClaimPayload =
        c.claimKind === 'reported_fact'
          ? { kind: 'reported_fact', predicate: c.predicate.trim(), value: c.value }
          : { kind: 'user_assessment', statement: c.statement.trim(), stance: 'endorsed' };
      const normalized =
        c.claimKind === 'reported_fact'
          ? `${c.predicate.trim()}=${String(c.value)}`
          : c.statement.trim();
      return {
        draft_id: crypto.randomUUID(),
        payload,
        normalized_draft: normalized,
        source_role: 'owner_explicit',
        evidence_excerpt: c.excerpt.trim(),
        decision: 'pending',
      };
    });
    const body: Partial<CaptureCandidate> = {
      claim_drafts: claimArr,
      ...(relationshipDraft
        ? { relationship_draft: { draft_id: crypto.randomUUID(), status: relationshipDraft.status, decision: 'pending' } }
        : {}),
      ...(interactionDraft
        ? {
            interaction_draft: {
              draft_id: crypto.randomUUID(),
              event_kind: interactionDraft.eventKind.trim() || 'note',
              headline: interactionDraft.headline.trim(),
              occurred_at: interactionDraft.occurredAt.trim()
                ? { kind: 'exact' as const, value: interactionDraft.occurredAt.trim() }
                : undefined,
              importance_or_topic: interactionDraft.importance.trim() || undefined,
              decision: 'pending',
            },
          }
        : {}),
      ...(isNew
        ? {
            person_draft: {
              display_name: capturePerson.displayName.trim(),
              private_aliases: capturePerson.aliases
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean),
            },
          }
        : { target_person_id: capturePerson.targetPersonID }),
    };
    setBusy(true);
    try {
      await proposeCandidate(body);
      setShowCapture(false);
      resetCapture();
      setTab('candidates');
      await loadCandidates();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'propose failed');
    } finally {
      setBusy(false);
    }
  }, [capturePerson, claimDrafts, relationshipDraft, interactionDraft, loadCandidates, resetCapture]);

  const TabButton = ({ id, label }: { id: Tab; label: string }) => (
    <button
      onClick={() => setTab(id)}
      className={`px-3 py-1.5 rounded-lg text-[12px] transition ${
        tab === id ? 'bg-amber-500/15 text-amber-200' : 'text-slate-400 hover:bg-slate-900'
      }`}
    >
      {label}
    </button>
  );

  const inputCls =
    'px-2 py-1 rounded-lg bg-slate-950 border border-slate-700 text-slate-200 text-[12px] focus:border-amber-500 outline-none w-full';

  return (
    <div className="h-full flex flex-col min-h-0 overflow-hidden bg-slate-950">
      <div className="px-4 py-3 border-b border-slate-800/80 flex items-center justify-between shrink-0">
        <div>
          <h2 className="text-base font-bold text-slate-100">
            <i className="fa-solid fa-users text-amber-400 mr-2"></i>
            {t('people.title', '人物与关系记忆')}
            <span
              className={`ml-2 align-middle inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full border ${
                liveState === 'open'
                  ? 'border-emerald-500/40 text-emerald-300'
                  : liveState === 'error'
                    ? 'border-rose-500/40 text-rose-300'
                    : 'border-slate-600 text-slate-400'
              }`}
              title={t('people.live.title', '跨标签页实时同步：其他标签的审批 / 折叠会即时反映在这里')}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  liveState === 'open'
                    ? 'bg-emerald-400 animate-pulse'
                    : liveState === 'error'
                      ? 'bg-rose-400'
                      : 'bg-slate-500'
                }`}
              ></span>
              {liveState === 'open'
                ? t('people.live.on', '实时')
                : liveState === 'error'
                  ? t('people.live.error', '离线')
                  : t('people.live.connecting', '连接中')}
            </span>
          </h2>
          <p className="text-[11px] text-slate-500 mt-0.5">
            {t('people.subtitle', 'Persistent Identity F276 · owner-private 第三方人物 / 关系 / 互动')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 text-[11px] text-slate-400" title={t('people.operator.title', '按 operator 作用域隔离（对应 ownerUserId）')}>
            <i className="fa-solid fa-user-shield mr-1 text-amber-400"></i>
            <input
              list="pm-operators"
              value={operatorInput}
              onChange={(e) => setOperatorInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') applyOperator(operatorInput);
              }}
              placeholder={t('people.operator.placeholder', 'operator（默认 leader）')}
              className="w-40 px-2 py-1 rounded-lg bg-slate-900 border border-slate-700 text-slate-200 text-[11px] focus:border-amber-500 outline-none"
            />
            <datalist id="pm-operators">
              {operators.map((o) => (
                <option key={o} value={o} />
              ))}
            </datalist>
            <button
              onClick={() => applyOperator(operatorInput)}
              className="px-2 py-1 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-300 text-[11px]"
            >
              {t('people.operator.apply', '应用')}
            </button>
            {getActiveOperator() && (
              <button
                onClick={() => applyOperator('')}
                title={t('people.operator.clear', '清除，回到 leader 默认作用域')}
                className="px-2 py-1 rounded-lg border border-slate-700 hover:border-slate-500 text-slate-400 text-[11px]"
              >
                ✕
              </button>
            )}
          </div>
          <TabButton id="people" label={t('people.tab.people', '人物')} />
          <TabButton id="candidates" label={t('people.tab.candidates', '待批候选')} />
          <TabButton id="deferred" label={t('people.tab.deferred', '延迟回执')} />
          <button
            onClick={() => toggleCapture(!showCapture)}
            className={`px-2 py-1 rounded-lg border text-[11px] ${
              showCapture
                ? 'border-amber-500 text-amber-200'
                : 'border-slate-700 hover:border-amber-500 text-slate-300'
            }`}
          >
            <i className="fa-solid fa-plus mr-1"></i>
            {t('people.capture.toggle', '新建捕获')}
          </button>
          <button
            onClick={reloadCurrent}
            className="ml-2 px-2 py-1 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-300 text-[11px]"
          >
            <i className="fa-solid fa-rotate-right mr-1"></i>
            {t('people.refresh', '刷新')}
          </button>
        </div>
      </div>

      {showCapture && (
        <div className="px-4 py-3 border-b border-slate-800/80 bg-slate-900/40 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-[12px] font-semibold text-amber-300">
              <i className="fa-solid fa-plus mr-1"></i>
              {t('people.capture.title', '新建人物捕获（多草稿待批候选）')}
            </h3>
            <button onClick={() => toggleCapture(false)} className="text-slate-400 text-[11px] hover:text-slate-200">
              {t('people.capture.collapse', '收起')}
            </button>
          </div>

          {/* Identity: new person or attach to existing */}
          <div className="grid grid-cols-2 gap-2 text-[12px]">
            <label className="flex flex-col gap-1 col-span-2">
              <span className="text-slate-400">{t('people.capture.target', '目标人物')}</span>
              <select
                value={capturePerson.targetPersonID}
                onChange={(e) => setCapturePerson({ ...capturePerson, targetPersonID: e.target.value })}
                className={inputCls}
              >
                <option value="">＋ {t('people.capture.newPerson', '新建人物')}</option>
                {people.map((p) => (
                  <option key={p.person_id} value={p.person_id}>
                    {p.display_name}
                  </option>
                ))}
              </select>
            </label>
            {!capturePerson.targetPersonID && (
              <>
                <label className="flex flex-col gap-1">
                  <span className="text-slate-400">{t('people.capture.name', '显示名 *')}</span>
                  <input
                    value={capturePerson.displayName}
                    onChange={(e) => setCapturePerson({ ...capturePerson, displayName: e.target.value })}
                    className={inputCls}
                  />
                </label>
                <label className="flex flex-col gap-1">
                  <span className="text-slate-400">{t('people.capture.aliases', '别名（逗号分隔，可选）')}</span>
                  <input
                    value={capturePerson.aliases}
                    onChange={(e) => setCapturePerson({ ...capturePerson, aliases: e.target.value })}
                    className={inputCls}
                  />
                </label>
              </>
            )}
          </div>

          {/* Claim draft cards */}
          {claimDrafts.map((c, i) => (
            <div key={i} className="rounded-xl border border-slate-700 bg-slate-950/40 p-2 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold text-amber-300">
                  <i className="fa-solid fa-file-lines mr-1"></i>
                  {t('people.capture.claim', '事实/评价')} #{i + 1}
                </span>
                <button
                  onClick={() => setClaimDrafts((prev) => prev.filter((_, idx) => idx !== i))}
                  className="text-slate-400 text-[11px] hover:text-rose-300"
                  title={t('people.capture.remove', '移除该草稿')}
                >
                  ✕
                </button>
              </div>
              <div className="grid grid-cols-2 gap-2 text-[12px]">
                <label className="flex flex-col gap-1">
                  <span className="text-slate-400">{t('people.capture.kind', '类型')}</span>
                  <select
                    value={c.claimKind}
                    onChange={(e) =>
                      setClaimDrafts((prev) =>
                        prev.map((x, idx) =>
                          idx === i ? { ...x, claimKind: e.target.value as 'reported_fact' | 'user_assessment' } : x,
                        ),
                      )
                    }
                    className={inputCls}
                  >
                    <option value="reported_fact">{t('people.capture.kind.fact', '事实 (reported_fact)')}</option>
                    <option value="user_assessment">{t('people.capture.kind.assessment', '评价 (user_assessment)')}</option>
                  </select>
                </label>
                {c.claimKind === 'reported_fact' ? (
                  <>
                    <label className="flex flex-col gap-1">
                      <span className="text-slate-400">predicate</span>
                      <input
                        value={c.predicate}
                        onChange={(e) =>
                          setClaimDrafts((prev) => prev.map((x, idx) => (idx === i ? { ...x, predicate: e.target.value } : x)))
                        }
                        className={inputCls}
                      />
                    </label>
                    <label className="flex flex-col gap-1">
                      <span className="text-slate-400">value</span>
                      <input
                        value={c.value}
                        onChange={(e) =>
                          setClaimDrafts((prev) => prev.map((x, idx) => (idx === i ? { ...x, value: e.target.value } : x)))
                        }
                        className={inputCls}
                      />
                    </label>
                  </>
                ) : (
                  <label className="flex flex-col gap-1 col-span-2">
                    <span className="text-slate-400">statement</span>
                    <input
                      value={c.statement}
                      onChange={(e) =>
                        setClaimDrafts((prev) => prev.map((x, idx) => (idx === i ? { ...x, statement: e.target.value } : x)))
                      }
                      className={inputCls}
                    />
                  </label>
                )}
                <label className="flex flex-col gap-1 col-span-2">
                  <span className="text-slate-400">{t('people.capture.excerpt', '证据摘录（可选）')}</span>
                  <input
                    value={c.excerpt}
                    onChange={(e) =>
                      setClaimDrafts((prev) => prev.map((x, idx) => (idx === i ? { ...x, excerpt: e.target.value } : x)))
                    }
                    className={inputCls}
                  />
                </label>
              </div>
            </div>
          ))}

          {/* Relationship draft card (at most one) */}
          {relationshipDraft && (
            <div className="rounded-xl border border-slate-700 bg-slate-950/40 p-2 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold text-sky-300">
                  <i className="fa-solid fa-link mr-1"></i>
                  {t('people.capture.relationship', '关系草稿')}
                </span>
                <button onClick={() => setRelationshipDraft(null)} className="text-slate-400 text-[11px] hover:text-rose-300">
                  ✕
                </button>
              </div>
              <label className="flex flex-col gap-1 text-[12px]">
                <span className="text-slate-400">{t('people.capture.relStatus', '关系状态')}</span>
                <select
                  value={relationshipDraft.status}
                  onChange={(e) => setRelationshipDraft({ status: e.target.value as 'current' | 'former' | 'unknown' })}
                  className={inputCls}
                >
                  <option value="current">{t('people.capture.rel.current', '当前 (current)')}</option>
                  <option value="former">{t('people.capture.rel.former', '前任 (former)')}</option>
                  <option value="unknown">{t('people.capture.rel.unknown', '未知 (unknown)')}</option>
                </select>
              </label>
            </div>
          )}

          {/* Interaction draft card (at most one) */}
          {interactionDraft && (
            <div className="rounded-xl border border-slate-700 bg-slate-950/40 p-2 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-semibold text-emerald-300">
                  <i className="fa-solid fa-calendar-day mr-1"></i>
                  {t('people.capture.interaction', '互动草稿')}
                </span>
                <button onClick={() => setInteractionDraft(null)} className="text-slate-400 text-[11px] hover:text-rose-300">
                  ✕
                </button>
              </div>
              <div className="grid grid-cols-2 gap-2 text-[12px]">
                <label className="flex flex-col gap-1">
                  <span className="text-slate-400">event_kind</span>
                  <input
                    value={interactionDraft.eventKind}
                    onChange={(e) => setInteractionDraft({ ...interactionDraft, eventKind: e.target.value })}
                    className={inputCls}
                  />
                </label>
                <label className="flex flex-col gap-1">
                  <span className="text-slate-400">{t('people.capture.occurredAt', '发生时间（可选）')}</span>
                  <input
                    value={interactionDraft.occurredAt}
                    onChange={(e) => setInteractionDraft({ ...interactionDraft, occurredAt: e.target.value })}
                    className={inputCls}
                  />
                </label>
                <label className="flex flex-col gap-1 col-span-2">
                  <span className="text-slate-400">headline</span>
                  <input
                    value={interactionDraft.headline}
                    onChange={(e) => setInteractionDraft({ ...interactionDraft, headline: e.target.value })}
                    className={inputCls}
                  />
                </label>
                <label className="flex flex-col gap-1 col-span-2">
                  <span className="text-slate-400">{t('people.capture.importance', '重要性 / 主题（可选）')}</span>
                  <input
                    value={interactionDraft.importance}
                    onChange={(e) => setInteractionDraft({ ...interactionDraft, importance: e.target.value })}
                    className={inputCls}
                  />
                </label>
              </div>
            </div>
          )}

          {/* Add-draft buttons */}
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setClaimDrafts((prev) => [...prev, emptyClaimDraft()])}
              className="px-2 py-1 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-300 text-[11px]"
            >
              <i className="fa-solid fa-plus mr-1"></i>
              {t('people.capture.addClaim', '添加事实/评价')}
            </button>
            {!relationshipDraft && (
              <button
                onClick={() => setRelationshipDraft({ status: 'current' })}
                className="px-2 py-1 rounded-lg border border-slate-700 hover:border-sky-500 text-slate-300 text-[11px]"
              >
                <i className="fa-solid fa-plus mr-1"></i>
                {t('people.capture.addRel', '添加关系草稿')}
              </button>
            )}
            {!interactionDraft && (
              <button
                onClick={() => setInteractionDraft({ eventKind: '', headline: '', occurredAt: '', importance: '' })}
                className="px-2 py-1 rounded-lg border border-slate-700 hover:border-emerald-500 text-slate-300 text-[11px]"
              >
                <i className="fa-solid fa-plus mr-1"></i>
                {t('people.capture.addEvent', '添加互动草稿')}
              </button>
            )}
          </div>

          <div className="flex gap-2 items-center">
            <button
              disabled={busy}
              onClick={submitCapture}
              className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-[12px] disabled:opacity-40"
            >
              <i className="fa-solid fa-paper-plane mr-1"></i>
              {t('people.capture.submit', '提交为候选')}
            </button>
            <span className="text-[10px] text-slate-500">
              {t('people.capture.hint', '将作为当前 operator 作用域下的多草稿待批候选出现于「待批候选」')}
            </span>
          </div>
        </div>
      )}

      <div className="flex-1 min-h-0 flex">
        <aside className="w-44 shrink-0 border-r border-slate-800/80 overflow-auto p-2 space-y-1">
          {tab === 'people' &&
            people.map((p) => (
              <button
                key={p.person_id}
                onClick={() => openPerson(p.person_id)}
                className={`w-full text-left px-2 py-1.5 rounded-lg text-[12px] flex items-center justify-between gap-1 transition ${
                  selectedPerson === p.person_id ? 'bg-amber-500/15 text-amber-200' : 'text-slate-300 hover:bg-slate-900'
                }`}
              >
                <span className="truncate">{p.display_name}</span>
                <span className="text-[10px] text-slate-500 shrink-0">{p.current_claims}✓</span>
              </button>
            ))}
          {tab === 'candidates' &&
            candidates.map((c) => (
              <button
                key={c.candidate_id}
                onClick={() => openCandidate(c.candidate_id)}
                className={`w-full text-left px-2 py-1.5 rounded-lg text-[12px] flex items-center justify-between gap-1 transition ${
                  selectedCand === c.candidate_id ? 'bg-amber-500/15 text-amber-200' : 'text-slate-300 hover:bg-slate-900'
                }`}
              >
                <span className="truncate">
                  {c.person_draft?.display_name ?? c.target_person_id ?? c.candidate_id}
                </span>
                <span className="text-[10px] text-slate-500 shrink-0">{c.state}</span>
              </button>
            ))}
          {tab === 'deferred' &&
            deferred.map((r) => (
              <div
                key={r.receipt_id}
                className="w-full text-left px-2 py-1.5 rounded-lg text-[12px] text-slate-300 hover:bg-slate-900 truncate"
              >
                {r.subject}
              </div>
            ))}
          {((tab === 'people' && !people.length) ||
            (tab === 'candidates' && !candidates.length) ||
            (tab === 'deferred' && !deferred.length)) && (
            <p className="text-[11px] text-slate-500 px-2 py-1">{t('people.empty', '暂无')}</p>
          )}
        </aside>

        <main className="flex-1 min-w-0 overflow-auto p-4">
          {loading && <p className="text-[12px] text-slate-500">{t('people.loading', '加载中…')}</p>}
          {error && <p className="text-[12px] text-rose-400">{error}</p>}
          {detail && <PersonDetailView detail={detail} onChanged={reloadCurrent} />}
          {cand && <CandidateDetailView candidate={cand} onChanged={reloadCurrent} />}
          {tab === 'deferred' && deferred.length > 0 && !cand && (
            <DeferredListView receipts={deferred} onChanged={reloadCurrent} />
          )}
        </main>
      </div>
    </div>
  );
}

function PersonDetailView({
  detail,
  onChanged,
}: {
  detail: PersonDetail;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const person = detail.person;

  const doForget = async () => {
    if (!confirm(`确认 hard-forget 整个「${person.display_name}」关系？此操作不可恢复。`)) return;
    setBusy(true);
    try {
      await forgetPerson(person.person_id);
      onChanged();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-bold text-slate-100">{person.display_name}</h3>
          <p className="text-[11px] text-slate-500">
            {person.private_aliases.join(' · ')} · {person.status}
          </p>
        </div>
        <button
          onClick={doForget}
          disabled={busy}
          className="px-3 py-1.5 rounded-lg border border-rose-700/60 text-rose-300 hover:bg-rose-900/20 text-[11px]"
        >
          <i className="fa-solid fa-trash mr-1"></i> hard-forget
        </button>
      </div>

      {detail.card && (
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-3">
          <div className="text-[11px] uppercase tracking-wider text-amber-400/80 mb-1">Recall Card（≤160 tokens）</div>
          <div className="text-[13px] text-slate-200 space-y-1">
            {detail.card.relationship_line && <div className="text-slate-400">{detail.card.relationship_line}</div>}
            {detail.card.facts.map((f) => (
              <div key={f.claim_id} className="flex gap-2">
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 shrink-0">
                  {f.kind === 'reported_fact' ? '事实' : '评价'}
                </span>
                <span>{f.text}</span>
              </div>
            ))}
            {detail.card.latest_interaction && (
              <div className="text-slate-400 text-[12px]">最近互动：{detail.card.latest_interaction.headline}</div>
            )}
          </div>
        </div>
      )}

      <Section title={`事实 / 评价（${detail.claims.length}）`}>
        {detail.claims.map((c) => (
          <ClaimRow key={c.claim_id} claim={c} personID={person.person_id} onChanged={onChanged} />
        ))}
      </Section>

      <Section title={`关系（${detail.relationships.length}）`}>
        {detail.relationships.map((r) => (
          <div key={r.relationship_id} className="text-[12px] text-slate-300 py-1">
            状态：{r.status} · {r.transitions.length} 次状态变迁
          </div>
        ))}
      </Section>

      <Section title={`互动事件（${detail.events.length}）`}>
        {detail.events.map((e) => (
          <EventRow key={e.event_id} event={e} personID={person.person_id} onChanged={onChanged} />
        ))}
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h4 className="text-[12px] font-semibold text-slate-400 mb-1">{title}</h4>
      <div className="rounded-xl border border-slate-800 bg-slate-900/40 divide-y divide-slate-800/70">{children}</div>
    </div>
  );
}

function ClaimRow({ claim, personID, onChanged }: { claim: PersonClaimVersion; personID: string; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const statusColor =
    claim.status === 'current' ? 'text-emerald-400' : claim.status === 'superseded' ? 'text-slate-500' : 'text-rose-400';

  return (
    <div className="px-3 py-2 flex items-center justify-between gap-2">
      <div className="min-w-0">
        <div className="text-[12px] text-slate-200 truncate">{claimText(claim)}</div>
        <div className={`text-[10px] ${statusColor}`}>{claim.status}</div>
      </div>
      {claim.status === 'current' && (
        <div className="flex gap-1 shrink-0">
          <button
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              const text = prompt('纠正后的内容（fact: 谓语=值 / assessment: 评价文本）');
              if (text) {
                const payload =
                  claim.payload.kind === 'reported_fact'
                    ? { kind: 'reported_fact', predicate: claim.payload.predicate, value: text }
                    : { kind: 'user_assessment', statement: text, stance: 'endorsed' as const };
                await correctClaim(personID, claim.claim_id, payload as any, { source_kind: 'operator' });
              }
              setBusy(false);
              onChanged();
            }}
            className="px-2 py-0.5 rounded border border-slate-700 text-slate-300 text-[10px] hover:border-amber-500"
          >
            纠正
          </button>
          <button
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              await retireClaim(personID, claim.claim_id, { source_kind: 'operator' });
              setBusy(false);
              onChanged();
            }}
            className="px-2 py-0.5 rounded border border-slate-700 text-slate-300 text-[10px] hover:border-slate-500"
          >
            退役
          </button>
          <button
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              await redactItem(personID, 'claim', claim.claim_id);
              setBusy(false);
              onChanged();
            }}
            className="px-2 py-0.5 rounded border border-slate-700 text-rose-300 text-[10px] hover:border-rose-500"
          >
            脱敏
          </button>
        </div>
      )}
    </div>
  );
}

function EventRow({ event, personID, onChanged }: { event: InteractionEvent; personID: string; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  return (
    <div className="px-3 py-2 flex items-center justify-between gap-2">
      <div className="min-w-0">
        <div className="text-[12px] text-slate-200 truncate">{event.headline}</div>
        <div className="text-[10px] text-slate-500">
          {event.event_kind} · {temporalLabel(event.occurred_at)}
        </div>
      </div>
      <div className="flex gap-1 shrink-0">
        <button
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            const h = prompt('修正后的 headline');
            if (h) {
              await amendEvent(
                personID,
                event.event_id,
                { draft_id: 'x', event_kind: event.event_kind, headline: h, decision: 'pending' },
                { source_kind: 'operator' },
              );
            }
            setBusy(false);
            onChanged();
          }}
          className="px-2 py-0.5 rounded border border-slate-700 text-slate-300 text-[10px] hover:border-amber-500"
        >
          修正
        </button>
        <button
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            await redactItem(personID, 'event', event.event_id);
            setBusy(false);
            onChanged();
          }}
          className="px-2 py-0.5 rounded border border-slate-700 text-rose-300 text-[10px] hover:border-rose-500"
        >
          脱敏
        </button>
      </div>
    </div>
  );
}

function CandidateDetailView({ candidate, onChanged }: { candidate: CaptureCandidate; onChanged: () => void }) {
  const { t } = useI18n();
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

  // Every draft (claim / relationship / interaction) decides on its own —
  // the profile-update-actions: each card has independent approve/reject
  // and folds in place once decided.
  const allDrafts = [
    ...candidate.claim_drafts,
    ...(candidate.relationship_draft ? [candidate.relationship_draft] : []),
    ...(candidate.interaction_draft ? [candidate.interaction_draft] : []),
  ];
  const pendingIDs = allDrafts.filter((d) => d.decision === 'pending').map((d) => d.draft_id);

  const decide = async (ids: string[], kind: 'approve' | 'reject') => {
    if (!ids.length) return;
    setBusy(kind + ':' + ids.join(','));
    setError('');
    try {
      if (kind === 'approve') await approveCandidate(candidate.candidate_id, ids);
      else await rejectDrafts(candidate.candidate_id, ids);
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : '操作失败');
    } finally {
      setBusy('');
    }
  };

  const keyFor = (id: string) => 'approve:' + id;
  const rejFor = (id: string) => 'reject:' + id;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-bold text-slate-100">
          {candidate.person_draft?.display_name ?? candidate.target_person_id ?? candidate.candidate_id}
        </h3>
        <span className="text-[11px] px-2 py-0.5 rounded bg-slate-800 text-slate-300">{candidate.state}</span>
      </div>
      <p className="text-[10px] text-slate-500">
        {t('people.card.hint', '每张草稿可独立批准 / 驳回；决定后就地折叠为 ✅ 已批准 / 🚫 已驳回')}
      </p>

      {candidate.claim_drafts.map((d, i) => (
        <DraftCard
          key={d.draft_id}
          label={d.payload.kind === 'reported_fact' ? t('people.capture.kind.fact', '事实') : t('people.capture.kind.assessment', '评价')}
          title={`#${i + 1}`}
          text={claimText(d as any)}
          excerpt={d.evidence_excerpt}
          decision={d.decision}
          busy={busy === keyFor(d.draft_id) || busy === rejFor(d.draft_id)}
          onApprove={() => decide([d.draft_id], 'approve')}
          onReject={() => decide([d.draft_id], 'reject')}
        />
      ))}
      {candidate.relationship_draft && (
        <DraftCard
          key={candidate.relationship_draft.draft_id}
          label={t('people.capture.relationship', '关系')}
          title=""
          text={`状态: ${candidate.relationship_draft.status}`}
          excerpt=""
          decision={candidate.relationship_draft.decision}
          busy={busy === keyFor(candidate.relationship_draft.draft_id) || busy === rejFor(candidate.relationship_draft.draft_id)}
          onApprove={() => decide([candidate.relationship_draft!.draft_id], 'approve')}
          onReject={() => decide([candidate.relationship_draft!.draft_id], 'reject')}
        />
      )}
      {candidate.interaction_draft && (
        <DraftCard
          key={candidate.interaction_draft.draft_id}
          label={t('people.capture.interaction', '互动')}
          title=""
          text={candidate.interaction_draft.headline}
          excerpt=""
          decision={candidate.interaction_draft.decision}
          busy={busy === keyFor(candidate.interaction_draft.draft_id) || busy === rejFor(candidate.interaction_draft.draft_id)}
          onApprove={() => decide([candidate.interaction_draft!.draft_id], 'approve')}
          onReject={() => decide([candidate.interaction_draft!.draft_id], 'reject')}
        />
      )}

      {error && <p className="text-[11px] text-rose-400">{error}</p>}

      <div className="flex flex-wrap gap-2 pt-1">
        <button
          disabled={busy !== '' || pendingIDs.length === 0}
          onClick={() => decide(pendingIDs, 'approve')}
          className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-[12px] disabled:opacity-40"
        >
          <i className="fa-solid fa-check mr-1"></i> {t('people.card.approveAll', '全部批准')}
        </button>
        <button
          disabled={busy !== '' || pendingIDs.length === 0}
          onClick={() => decide(pendingIDs, 'reject')}
          className="px-3 py-1.5 rounded-lg border border-slate-700 text-slate-300 text-[12px] hover:border-rose-500 disabled:opacity-40"
        >
          <i className="fa-solid fa-xmark mr-1"></i> {t('people.card.rejectAll', '全部驳回')}
        </button>
        <button
          disabled={busy !== ''}
          onClick={async () => {
            setBusy('reject');
            await rejectCandidate(candidate.candidate_id);
            setBusy('');
            onChanged();
          }}
          className="px-3 py-1.5 rounded-lg border border-slate-700 text-slate-300 text-[12px] hover:border-rose-500"
        >
          {t('people.card.rejectCandidate', '拒绝候选')}
        </button>
        <button
          disabled={busy !== ''}
          onClick={async () => {
            setBusy('notnow');
            await notNowCandidate(candidate.candidate_id);
            setBusy('');
            onChanged();
          }}
          className="px-3 py-1.5 rounded-lg border border-slate-700 text-slate-300 text-[12px] hover:border-slate-500"
        >
          {t('people.card.notNow', '稍后')}
        </button>
        <button
          disabled={busy !== ''}
          onClick={async () => {
            setBusy('withdraw');
            await withdrawCandidate(candidate.candidate_id);
            setBusy('');
            onChanged();
          }}
          className="px-3 py-1.5 rounded-lg border border-slate-700 text-slate-300 text-[12px]"
        >
          {t('people.card.withdraw', '撤回')}
        </button>
        {candidate.decision_refs && candidate.decision_refs.length > 0 && (
          <button
            disabled={busy !== ''}
            onClick={async () => {
              setBusy('undo');
              await undoDecision(candidate.candidate_id, candidate.decision_refs![candidate.decision_refs!.length - 1]);
              setBusy('');
              onChanged();
            }}
            className="px-3 py-1.5 rounded-lg border border-slate-700 text-slate-300 text-[12px]"
          >
            {t('people.card.undo', '撤销审批')}
          </button>
        )}
      </div>
    </div>
  );
}

// A single draft as its own card. Pending: shows 批准/驳回. Decided: folds
// inline into a compact ✅ / 🚫 bar (profile-update-actions behaviour).
function DraftCard({
  label,
  title,
  text,
  excerpt,
  decision,
  busy,
  onApprove,
  onReject,
}: {
  label: string;
  title: string;
  text: string;
  excerpt: string;
  decision: 'pending' | 'approved' | 'rejected';
  busy: boolean;
  onApprove: () => void;
  onReject: () => void;
}) {
  if (decision === 'approved') {
    return (
      <div className="rounded-xl border border-emerald-500/40 bg-emerald-500/5 px-3 py-2 flex items-center gap-2 text-[12px] text-emerald-300">
        <i className="fa-solid fa-check shrink-0"></i>
        <span className="font-semibold shrink-0">{label}</span>
        <span className="text-slate-400 truncate">· {text}</span>
      </div>
    );
  }
  if (decision === 'rejected') {
    return (
      <div className="rounded-xl border border-rose-500/40 bg-rose-500/5 px-3 py-2 flex items-center gap-2 text-[12px] text-rose-300">
        <i className="fa-solid fa-ban shrink-0"></i>
        <span className="font-semibold shrink-0">{label}</span>
        <span className="text-slate-400 truncate">· {text}</span>
      </div>
    );
  }
  return (
    <div className="rounded-xl border border-slate-700 bg-slate-950/40 p-3 space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 w-fit">{label}</span>
          {title && <span className="text-[11px] text-slate-500">{title}</span>}
        </div>
        <div className="flex gap-1 shrink-0">
          <button
            disabled={busy}
            onClick={onApprove}
            className="px-2 py-0.5 rounded border border-emerald-600 text-emerald-300 text-[11px] hover:bg-emerald-900/20 disabled:opacity-40"
          >
            <i className="fa-solid fa-check mr-1"></i> 批准
          </button>
          <button
            disabled={busy}
            onClick={onReject}
            className="px-2 py-0.5 rounded border border-rose-700/60 text-rose-300 text-[11px] hover:bg-rose-900/20 disabled:opacity-40"
          >
            <i className="fa-solid fa-xmark mr-1"></i> 驳回
          </button>
        </div>
      </div>
      <div className="text-[12px] text-slate-200">{text}</div>
      {excerpt && <div className="text-[10px] text-slate-500">"{excerpt}"</div>}
    </div>
  );
}

function DeferredListView({
  receipts,
  onChanged,
}: {
  receipts: DeferredPersonMemoryReceipt[];
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState('');
  return (
    <div className="space-y-2">
      <p className="text-[11px] text-slate-500">延迟回执（content-free，由每日 clerk 转回普通候选）</p>
      {receipts.map((r) => (
        <div key={r.receipt_id} className="rounded-xl border border-slate-800 bg-slate-900/40 px-3 py-2 flex items-center justify-between gap-2">
          <div className="min-w-0">
            <div className="text-[12px] text-slate-200 truncate">{r.subject}</div>
            <div className="text-[10px] text-slate-500">{r.requester_dog} · {r.source_coords.length} 条来源坐标</div>
          </div>
          <div className="flex gap-1 shrink-0">
            <button
              disabled={busy === r.receipt_id}
              onClick={async () => {
                setBusy(r.receipt_id);
                await claimDeferred(r.receipt_id);
                setBusy('');
                onChanged();
              }}
              className="px-2 py-0.5 rounded border border-amber-600 text-amber-200 text-[10px]"
            >
              转为候选
            </button>
            <button
              disabled={busy === r.receipt_id}
              onClick={async () => {
                setBusy(r.receipt_id);
                await withdrawReceipt(r.receipt_id);
                setBusy('');
                onChanged();
              }}
              className="px-2 py-0.5 rounded border border-slate-700 text-slate-300 text-[10px]"
            >
              撤回
            </button>
            <button
              disabled={busy === r.receipt_id}
              onClick={async () => {
                setBusy(r.receipt_id);
                await forgetReceipt(r.receipt_id);
                setBusy('');
                onChanged();
              }}
              className="px-2 py-0.5 rounded border border-slate-700 text-rose-300 text-[10px]"
            >
              遗忘
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
