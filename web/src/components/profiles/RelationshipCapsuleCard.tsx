import { useI18n } from '../../store/useI18n';
import { breedLabel, breedDot } from './breedMeta';
import { ApprovalCard } from './ApprovalCard';
import { DistillControls } from './DistillControls';
import type { RelationshipCapsule } from '../../types/profiles';

interface Props {
  capsule: RelationshipCapsule;
  proposal: RelationshipCapsule | null;
  onChanged: () => void;
}

export function RelationshipCapsuleCard({ capsule, proposal, onChanged }: Props) {
  const { t } = useI18n();
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
        </div>
      </div>

      <div>
        <div className="text-[11px] text-slate-500 mb-1">{t('profiles.activeCapsule', '当前关系画像（active）')}</div>
        <p className="text-[12px] text-slate-200 whitespace-pre-wrap leading-relaxed bg-slate-950/60 rounded-lg p-3 border border-slate-800">
          {capsule.body || t('profiles.empty', '（空）')}
        </p>
      </div>

      {proposal ? (
        <ApprovalCard relationshipKey={capsule.relationship_key} proposal={proposal} onDecided={onChanged} />
      ) : (
        <DistillControls relationshipKey={capsule.relationship_key} onDistilled={onChanged} />
      )}
    </div>
  );
}
