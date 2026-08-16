import { useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { useAppStore } from '../../store/useAppStore';
import { distill, distillAgent } from '../../services/profilesService';
import { BREED_OPTIONS } from './breedMeta';
import type { DistillResult } from '../../types/profiles';

interface Props {
  relationshipKey: string;
  // called after a successful agent distill so the parent can reload the
  // newly-created pending proposal.
  onDistilled: () => void;
}

// Faithful to F231: the distiller is the dog of the CURRENT session
// (derived server-side from ?session_id). An explicit ?client_id=<breed> is an
// operator override. There is no hardcoded default dog — if neither is present
// the backend refuses with 400.
export function DistillControls({ relationshipKey, onDistilled }: Props) {
  const { t } = useI18n();
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [evidence, setEvidence] = useState<DistillResult | null>(null);
  const [clientId, setClientId] = useState(BREED_OPTIONS[0].id);

  const runAgent = async (opts: { sessionId?: string; clientId?: string }) => {
    setBusy(true);
    setError('');
    setEvidence(null);
    try {
      await distillAgent(relationshipKey, opts);
      onDistilled();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'distill failed');
    } finally {
      setBusy(false);
    }
  };

  const runEvidence = async () => {
    setBusy(true);
    setError('');
    try {
      const res = await distill(relationshipKey);
      setEvidence(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'distill failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2">
      <div className="text-[11px] text-slate-500">{t('profiles.distillSection', '养熟 · 蒸馏')}</div>
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => runAgent({ sessionId: activeThreadId })}
          disabled={busy || !activeThreadId}
          title={activeThreadId ? '' : t('profiles.noSession', '当前无活动会话，无法自动派生蒸馏者')}
          className="px-3 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-[11px] font-medium transition disabled:opacity-50"
        >
          <i className="fa-solid fa-paw mr-1"></i>
          {t('profiles.distillSession', '让当前会话的狗蒸馏')}
        </button>
        <button
          onClick={runEvidence}
          disabled={busy}
          className="px-3 py-1.5 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-200 text-[11px] transition disabled:opacity-50"
        >
          <i className="fa-solid fa-list mr-1"></i>
          {t('profiles.evidenceOnly', '仅聚合证据')}
        </button>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <select
          value={clientId}
          onChange={(e) => setClientId(e.target.value)}
          className="bg-slate-950 border border-slate-800 rounded-lg px-2 py-1.5 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500"
        >
          {BREED_OPTIONS.map((b) => (
            <option key={b.id} value={b.id}>
              {b.label}
            </option>
          ))}
        </select>
        <button
          onClick={() => runAgent({ clientId })}
          disabled={busy}
          className="px-3 py-1.5 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-200 text-[11px] transition disabled:opacity-50"
        >
          <i className="fa-solid fa-dog mr-1"></i>
          {t('profiles.distillBreed', '指定狗狗蒸馏')}
        </button>
      </div>
      {error && <p className="text-[11px] text-rose-400">{error}</p>}
      {evidence && (
        <div className="rounded-xl border border-slate-800 bg-slate-900/60 p-3">
          <div className="text-[11px] text-slate-400 mb-1">
            {t('profiles.evidenceCount', `相关证据 ${evidence.evidence_count} 条`)}
          </div>
          <ul className="space-y-1 max-h-48 overflow-auto">
            {evidence.evidence.map((ev) => (
              <li key={ev.id} className="text-[11px] text-slate-300 border-l-2 border-slate-700 pl-2">
                <span className="text-slate-500">[{ev.type}]</span> {ev.title}：{ev.content}
              </li>
            ))}
          </ul>
        </div>
      )}
      <p className="text-[10px] text-slate-500">
        {t('profiles.distillHint', '蒸馏者由当前会话的狗自动派生；可选指定狗狗覆盖。草稿落为待审提案，需你批准才写入。')}
      </p>
    </div>
  );
}
