import type { SopGateEvent } from '../../types';
import { useI18n } from '../../store/useI18n';

interface SopGateProps {
  event: SopGateEvent;
}

export function SopGate({ event }: SopGateProps) {
  const { t } = useI18n();
  return (
    <div className="my-2 bg-amber-950/30 border border-amber-500/30 rounded-xl p-3 flex items-center justify-between text-xs text-amber-200">
      <div className="flex items-center space-x-2.5">
        <i className="fa-solid fa-shield-halved text-amber-400 text-sm"></i>
        <div>
          <span className="font-bold">[SOP Gate Triggered]:</span>
          <span className="text-amber-300/90 ml-1">{event.reason ?? t('workspace.sopGate.defaultReason')}</span>
        </div>
      </div>
      <span className="font-mono text-[10px] bg-amber-500/20 px-2 py-0.5 rounded border border-amber-500/30 text-amber-300">Rule: SOP-CROSS-REVIEW-02</span>
    </div>
  );
}
