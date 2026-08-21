import { useCallback, useEffect, useRef, useState } from 'react';
import type { WorkflowCheckStatus, WorkflowSopState } from '../../types/api';
import { apiGet } from '../../services/http';
import { useI18n } from '../../store/useI18n';

/**
 * Development SOP stage order (matches packs/default/sop/development.yaml).
 * fresh_context is optional: a feature may skip it (quality_gate -> review).
 */
const STAGE_ORDER = [
  'kickoff',
  'impl',
  'quality_gate',
  'fresh_context',
  'review',
  'merge',
  'completion',
] as const;

const STAGE_LABELS: Record<string, string> = {
  kickoff: 'Kickoff',
  impl: 'Impl',
  quality_gate: 'Quality Gate',
  fresh_context: 'Fresh Context',
  review: 'Review',
  merge: 'Merge',
  completion: 'Completion',
};

interface WorkflowSopPanelProps {
  featureId: string;
}

function CheckBadge({ status }: { status: WorkflowCheckStatus }) {
  if (status === 'verified') {
    return (
      <span className="rounded-full bg-emerald-500/20 border border-emerald-500/30 px-2 py-0.5 text-[10px] font-medium text-emerald-300">
        verified
      </span>
    );
  }
  if (status === 'attested') {
    return (
      <span className="rounded-full bg-amber-500/20 border border-amber-500/30 px-2 py-0.5 text-[10px] font-medium text-amber-300">
        attested
      </span>
    );
  }
  return (
    <span className="rounded-full bg-slate-700/40 border border-slate-600/40 px-2 py-0.5 text-[10px] font-medium text-slate-400">
      unknown
    </span>
  );
}

function StagePills({ current }: { current: string }) {
  const currentIdx = STAGE_ORDER.indexOf(current as (typeof STAGE_ORDER)[number]);
  return (
    <div className="flex flex-wrap gap-1">
      {STAGE_ORDER.map((stage, idx) => {
        const isCurrent = stage === current;
        const isPast = currentIdx >= 0 && idx < currentIdx;
        let className =
          'rounded-full px-2 py-0.5 text-[10px] font-medium transition-colors border';
        if (isCurrent) {
          className += ' bg-amber-500 text-slate-950 border-amber-400 font-semibold';
        } else if (isPast) {
          className += ' bg-slate-800/60 text-slate-400 border-slate-700/60';
        } else {
          className += ' bg-slate-900/40 text-slate-500 border-slate-800/60';
        }
        return (
          <span key={stage} className={className}>
            {STAGE_LABELS[stage] ?? stage}
          </span>
        );
      })}
    </div>
  );
}

function formatUpdatedAt(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * WorkflowSopPanel renders the SOP bulletin board for one feature: stage
 * pills, baton holder, next skill, resume capsule and check attestations.
 * The board is information sharing, not flow control — dogs decide their own
 * actions; the panel just makes the handoff state visible.
 */
export function WorkflowSopPanel({ featureId }: WorkflowSopPanelProps) {
  const { t } = useI18n();
  const [sop, setSop] = useState<WorkflowSopState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestSeq = useRef(0);

  const loadSop = useCallback(async (fid: string) => {
    const seq = ++requestSeq.current;
    setLoading(true);
    setError(null);
    try {
      const data = await apiGet<WorkflowSopState>(
        `/api/backlog/${encodeURIComponent(fid)}/workflow-sop`,
      );
      if (seq !== requestSeq.current) return;
      setSop(data);
    } catch (err) {
      if (seq !== requestSeq.current) return;
      const status = (err as { status?: number }).status;
      if (status === 404) {
        setSop(null);
      } else {
        setError(err instanceof Error ? err.message : String(err));
        setSop(null);
      }
    } finally {
      if (seq === requestSeq.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!featureId.trim()) {
      setSop(null);
      setError(null);
      return;
    }
    void loadSop(featureId.trim());
  }, [featureId, loadSop]);

  return (
    <section className="rounded-xl bg-amber-950/30 border border-amber-500/30 p-3 text-xs" data-testid="workflow-sop-panel">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-amber-200">
          <i className="fa-solid fa-clipboard-list text-amber-400 mr-1.5"></i>
          {t('workspace.sopPanel.title')}
        </h2>
        <span className="font-mono text-[10px] text-amber-300/70">{featureId}</span>
      </div>

      {!featureId.trim() ? (
        <p className="text-slate-400">{t('workspace.sopPanel.empty')}</p>
      ) : loading ? (
        <p className="text-slate-400">{t('workspace.sopPanel.loading')}</p>
      ) : error ? (
        <p className="rounded-lg bg-red-950/40 border border-red-500/30 px-2 py-2 text-red-300">{error}</p>
      ) : !sop ? (
        <p className="rounded-lg bg-slate-900/60 border border-slate-800 px-2 py-2 text-slate-400">
          {t('workspace.sopPanel.noBoard')}
        </p>
      ) : (
        <>
          <div className="mb-3">
            <StagePills current={sop.stage} />
          </div>

          <div className="mb-3 rounded-lg bg-slate-900/60 border border-slate-800 px-2.5 py-2 space-y-1">
            <p className="text-slate-400">
              {t('workspace.sopPanel.batonHolder')}
              <span className="font-semibold text-amber-200 ml-1">{sop.baton_holder || '—'}</span>
            </p>
            <p className="text-slate-400">
              {t('workspace.sopPanel.nextSkill')}
              <span className="font-medium text-slate-300 ml-1">{sop.next_skill || '—'}</span>
            </p>
          </div>

          {sop.resume_capsule && (
            <div className="mb-3 rounded-lg bg-slate-900/60 border border-slate-800 px-2.5 py-2">
              <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-amber-400/80">
                {t('workspace.sopPanel.resumeCapsule')}
              </p>
              <p className="text-slate-300 whitespace-pre-wrap">{sop.resume_capsule}</p>
            </div>
          )}

          <div className="mb-2 space-y-1">
            <p className="text-[10px] font-semibold uppercase tracking-wide text-amber-400/80">
              {t('workspace.sopPanel.checks')}
            </p>
            {sop.checks.length === 0 ? (
              <p className="text-slate-500">{t('workspace.sopPanel.noChecks')}</p>
            ) : (
              sop.checks.map((c) => (
                <div key={c.name} className="flex items-center justify-between">
                  <span className="text-slate-400">{c.name}</span>
                  <CheckBadge status={c.status} />
                </div>
              ))
            )}
          </div>

          <div className="border-t border-slate-800/80 pt-1.5">
            <p className="text-[10px] text-slate-500">
              {t('workspace.sopPanel.updatedAt')} {formatUpdatedAt(sop.updated_at)}
            </p>
          </div>
        </>
      )}
    </section>
  );
}
