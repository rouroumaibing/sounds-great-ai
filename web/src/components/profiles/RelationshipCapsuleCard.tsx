import { useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { breedLabel, breedDot } from './breedMeta';
import { ApprovalCard } from './ApprovalCard';
import { DistillControls } from './DistillControls';
import { proposeCapsule, upsertCapsule, deleteCapsule } from '../../services/profilesService';
import type { RelationshipCapsule } from '../../types/profiles';

interface Props {
  capsule: RelationshipCapsule;
  proposal: RelationshipCapsule | null;
  onChanged: () => void;
}

// CapsuleEditor lets the operator author a capsule update in place. 提案走
// 审批流（propose → approve）；直接写入跳过审批，仅限 operator 明确选择时。
function CapsuleEditor({ relationshipKey, onChanged }: { relationshipKey: string; onChanged: () => void }) {
  const { t } = useI18n();
  const [body, setBody] = useState('');
  const [busy, setBusy] = useState<'propose' | 'write' | null>(null);
  const [error, setError] = useState('');

  const submit = async (mode: 'propose' | 'write') => {
    if (!body.trim() || busy) return;
    setBusy(mode);
    setError('');
    try {
      if (mode === 'propose') await proposeCapsule(relationshipKey, body.trim());
      else await upsertCapsule(relationshipKey, body.trim());
      setBody('');
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'submit failed');
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="rounded-xl border border-slate-700/60 bg-slate-950/50 p-3 space-y-2">
      <div className="text-[11px] font-semibold text-slate-300">
        <i className="fa-solid fa-pen-nib mr-1 text-amber-400"></i>
        {t('profiles.editorTitle', '更新关系画像')}
      </div>
      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        rows={3}
        placeholder={t('profiles.editorPlaceholder', '写下你对这段关系的最新观察与期望…')}
        className="w-full rounded-lg bg-slate-950 border border-slate-800 px-2.5 py-1.5 text-[12px] text-slate-200 placeholder:text-slate-600 leading-relaxed outline-none focus:border-amber-500/60 resize-y"
      />
      {error && <p className="text-[11px] text-rose-400">{error}</p>}
      <div className="flex items-center gap-2">
        <button
          onClick={() => void submit('propose')}
          disabled={!body.trim() || busy !== null}
          className="px-3 py-1 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-[11px] font-medium transition disabled:opacity-40"
        >
          {busy === 'propose' ? t('profiles.working', '处理中…') : t('profiles.proposeAction', '提交提案（走审批）')}
        </button>
        <button
          onClick={() => void submit('write')}
          disabled={!body.trim() || busy !== null}
          className="px-3 py-1 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-100 text-[11px] font-medium transition disabled:opacity-40"
        >
          {busy === 'write' ? t('profiles.working', '处理中…') : t('profiles.writeDirect', '直接写入（跳过审批）')}
        </button>
      </div>
    </div>
  );
}

export function RelationshipCapsuleCard({ capsule, proposal, onChanged }: Props) {
  const { t } = useI18n();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const handleDelete = async () => {
    if (!confirmingDelete) {
      setConfirmingDelete(true);
      setTimeout(() => setConfirmingDelete(false), 3000);
      return;
    }
    try {
      await deleteCapsule(capsule.relationship_key);
      onChanged();
    } catch (e) {
      setDeleteError(e instanceof Error ? e.message : 'delete failed');
    }
  };

  return (
    <div className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-4 space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className={`w-2.5 h-2.5 rounded-full ${breedDot(capsule.owner_dog)}`}></span>
          <span className="text-sm font-semibold text-slate-100">{breedLabel(capsule.owner_dog)}</span>
          <span className="text-[10px] text-slate-500">· {capsule.status}</span>
        </div>
        <div className="flex items-center gap-2 text-[10px] text-slate-500">
          <span>✓ {capsule.eval_approvals}</span>
          <span className="text-rose-400">✕ {capsule.eval_rejections}</span>
          <button
            onClick={() => void handleDelete()}
            className={`ml-1 px-1.5 py-0.5 rounded transition text-[10px] ${
              confirmingDelete ? 'bg-rose-600 text-white' : 'text-slate-500 hover:text-rose-400'
            }`}
            title={t('profiles.deleteCapsule', '删除该关系键及待审提案')}
          >
            <i className="fa-regular fa-trash-can"></i>
            {confirmingDelete ? ` ${t('profiles.confirmDelete', '确认删除？')}` : ''}
          </button>
        </div>
      </div>
      {deleteError && <p className="text-[11px] text-rose-400">{deleteError}</p>}

      <div>
        <div className="text-[11px] text-slate-500 mb-1">{t('profiles.activeCapsule', '当前关系画像（active）')}</div>
        <p className="text-[12px] text-slate-200 whitespace-pre-wrap leading-relaxed bg-slate-950/60 rounded-lg p-3 border border-slate-800">
          {capsule.body || t('profiles.empty', '（空）')}
        </p>
      </div>

      {proposal ? (
        <ApprovalCard relationshipKey={capsule.relationship_key} proposal={proposal} onDecided={onChanged} />
      ) : (
        <>
          <CapsuleEditor relationshipKey={capsule.relationship_key} onChanged={onChanged} />
          <DistillControls relationshipKey={capsule.relationship_key} onDistilled={onChanged} />
        </>
      )}
    </div>
  );
}
