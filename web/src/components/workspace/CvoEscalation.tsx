import type { CvoEscalationEvent } from '../../types';
import { useI18n } from '../../store/useI18n';

export function CvoEscalation({ event }: { event: CvoEscalationEvent }) {
  const { t } = useI18n();
  const resolveEscalation = (_decision: string) => {};

  const title = event.escalationTitle ?? t('workspace.escalation.title');
  const optionA = event.options?.[0];
  const optionB = event.options?.[1];
  const optionAId = optionA?.id ?? 'option_1';
  const optionALabel = optionA?.label ?? t('workspace.escalation.optionA');
  const optionBId = optionB?.id ?? 'option_2';
  const optionBLabel = optionB?.label ?? t('workspace.escalation.optionB');

  return (
    <div className="my-3 bg-rose-950/40 border border-rose-500/60 rounded-2xl p-4 shadow-xl animate-pulse-border space-y-3">
      <div className="flex items-start justify-between">
        <div className="flex items-center space-x-2.5 text-rose-300">
          <div className="w-8 h-8 rounded-xl bg-rose-500/20 border border-rose-500/40 flex items-center justify-center text-rose-400 shrink-0">
            <i className="fa-solid fa-triangle-exclamation"></i>
          </div>
          <div>
            <h4 className="font-bold text-sm text-slate-100">A2A Depth Hard Rail Hit (max_a2a_depth = 3)</h4>
            <p className="text-xs text-rose-300/80">{title}{t('workspace.escalation.desc')}</p>
          </div>
        </div>
        <span className="font-mono text-[10px] bg-rose-500/20 text-rose-300 px-2 py-1 rounded-md border border-rose-500/40 shrink-0">ACTION REQUIRED</span>
      </div>

      {/* Options Buttons for CVO Intervention */}
      <div className="pt-2 border-t border-rose-900/50 flex flex-wrap items-center gap-2">
        <button onClick={() => resolveEscalation(optionAId)} className="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center gap-1.5 transition shadow-lg shadow-emerald-600/20">
          <i className="fa-solid fa-check"></i>
          <span>{optionALabel}</span>
        </button>
        <button onClick={() => resolveEscalation(optionBId)} className="px-3.5 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold flex items-center gap-1.5 transition shadow-lg shadow-cyan-600/20">
          <i className="fa-solid fa-code"></i>
          <span>{optionBLabel}</span>
        </button>
        <button onClick={() => resolveEscalation('intervene')} className="px-3.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold flex items-center gap-1.5 transition">
          <i className="fa-solid fa-terminal"></i>
          <span>{t('workspace.escalation.customCvo')}</span>
        </button>
      </div>
    </div>
  );
}
