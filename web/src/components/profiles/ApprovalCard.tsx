import { useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { approveProposal, rejectProposal } from '../../services/profilesService';
import { breedLabel, breedDot } from './breedMeta';
import type { RelationshipCapsule } from '../../types/profiles';

interface Props {
  relationshipKey: string;
  proposal: RelationshipCapsule;
  onDecided: () => void;
}

type Decision = 'pending' | 'busy' | 'approved' | 'rejected' | 'error';

// Faithful to profile-update-actions.ts: the proposal card renders
// approve/reject while pending, and on a terminal decision "collapses" in place
// into a single status line (✓ 已批准并写入 primer / 已驳回该提议) — no popup,
// no navigation. We mirror that exact state-machine + wording.
export function ApprovalCard({ relationshipKey, proposal, onDecided }: Props) {
  const { t } = useI18n();
  const [decision, setDecision] = useState<Decision>('pending');
  const [error, setError] = useState('');

  const handleApprove = async () => {
    setDecision('busy');
    try {
      await approveProposal(relationshipKey);
      setDecision('approved');
      onDecided();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'approve failed');
      setDecision('error');
    }
  };

  const handleReject = async () => {
    setDecision('busy');
    try {
      await rejectProposal(relationshipKey);
      setDecision('rejected');
      onDecided();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'reject failed');
      setDecision('error');
    }
  };

  if (decision === 'approved') {
    return (
      <div className="my-2 rounded-xl border border-emerald-500/40 bg-emerald-500/5 px-3 py-2 text-[12px] text-emerald-300">
        <i className="fa-solid fa-check mr-1"></i>
        {t('profiles.approved', '✓ 已批准并写入 primer')}
      </div>
    );
  }
  if (decision === 'rejected') {
    return (
      <div className="my-2 rounded-xl border border-rose-500/40 bg-rose-500/5 px-3 py-2 text-[12px] text-rose-300">
        <i className="fa-solid fa-xmark mr-1"></i>
        {t('profiles.rejected', '已驳回该提议')}
      </div>
    );
  }

  const busy = decision === 'busy';
  return (
    <div className="my-2 rounded-xl border border-amber-500/40 bg-amber-500/5 p-3 space-y-2">
      <div className="flex items-center gap-2 text-xs">
        <span className={`w-2 h-2 rounded-full ${breedDot(proposal.owner_dog)}`}></span>
        <span className="font-bold text-amber-300">{t('profiles.proposalTitle', '提议更新关系档案（primer）')}</span>
        <span className="text-slate-500 text-[10px]">· {breedLabel(proposal.owner_dog)}</span>
      </div>
      <p className="text-[12px] text-slate-200 whitespace-pre-wrap leading-relaxed">{proposal.body}</p>
      {error && <p className="text-[11px] text-rose-400">{error}</p>}
      <div className="flex items-center gap-2">
        <button
          onClick={handleApprove}
          disabled={busy}
          className="px-3 py-1 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-[11px] font-medium transition disabled:opacity-50"
        >
          <i className="fa-solid fa-check text-[9px] mr-1"></i>
          {t('profiles.approve', '批准并写入')}
        </button>
        <button
          onClick={handleReject}
          disabled={busy}
          className="px-3 py-1 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-[11px] font-medium transition disabled:opacity-50"
        >
          <i className="fa-solid fa-xmark text-[9px] mr-1"></i>
          {t('profiles.reject', '驳回')}
        </button>
        {busy && <span className="text-[11px] text-slate-400">{t('profiles.working', '处理中…')}</span>}
      </div>
    </div>
  );
}
