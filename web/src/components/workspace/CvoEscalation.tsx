import { useState } from 'react';
import type { CvoEscalationEvent } from '../../types';
import { useI18n } from '../../store/useI18n';
import { useChatStore } from '../../store/useChatStore';
import { useAppStore } from '../../store/useAppStore';

export function CvoEscalation({ event }: { event: CvoEscalationEvent }) {
  const { t } = useI18n();
  const resolveEscalation = useChatStore((s) => s.resolveEscalation);
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const [busy, setBusy] = useState(false);

  const threadId = event.threadId ?? activeThreadId;
  const railTitle = t('workspace.escalation.railTitle');
  const title = event.escalationTitle ?? t('workspace.escalation.title');
  const detail = event.reason ?? t('workspace.escalation.desc');
  const optionA = event.options?.find((o) => o.id === 'option_1') ?? event.options?.[0];
  const optionB = event.options?.find((o) => o.id === 'option_2') ?? event.options?.[1];
  const optionAId = optionA?.id ?? 'option_1';
  const optionALabel = optionA?.label ?? t('workspace.escalation.optionA');
  const optionBId = optionB?.id ?? 'option_2';
  const optionBLabel = optionB?.label ?? t('workspace.escalation.optionB');

  const decide = (decision: string) => {
    if (!threadId || busy) return;
    setBusy(true);
    // The backend re-dispatches option prompts; "intervene" resolves without
    // re-dispatch so the operator can type a custom directive in CommandBar.
    resolveEscalation(threadId, decision, event.escalationId);
  };

  return (
    <div className="my-3 bg-rose-950/40 border border-rose-500/60 rounded-2xl p-4 shadow-xl animate-pulse-border space-y-3">
      <div className="flex items-start justify-between">
        <div className="flex items-center space-x-2.5 text-rose-300">
          <div className="w-8 h-8 rounded-xl bg-rose-500/20 border border-rose-500/40 flex items-center justify-center text-rose-400 shrink-0">
            <i className="fa-solid fa-triangle-exclamation"></i>
          </div>
          <div>
            <h4 className="font-bold text-sm text-slate-100">
              {railTitle}
              {event.maxDepth ? <span className="font-mono text-[10px] text-rose-300/80 ml-1.5">(max_a2a_depth = {event.maxDepth})</span> : null}
            </h4>
            <p className="text-xs text-rose-300/80">{title} — {detail}</p>
          </div>
        </div>
        <span className="font-mono text-[10px] bg-rose-500/20 text-rose-300 px-2 py-1 rounded-md border border-rose-500/40 shrink-0">ACTION REQUIRED</span>
      </div>

      {/* Options Buttons for CVO Intervention */}
      <div className="pt-2 border-t border-rose-900/50 flex flex-wrap items-center gap-2">
        <button
          onClick={() => decide(optionAId)}
          disabled={busy}
          className="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center gap-1.5 transition shadow-lg shadow-emerald-600/20 disabled:opacity-50"
        >
          <i className="fa-solid fa-check"></i>
          <span>{optionALabel}</span>
        </button>
        <button
          onClick={() => decide(optionBId)}
          disabled={busy}
          className="px-3.5 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold flex items-center gap-1.5 transition shadow-lg shadow-cyan-600/20 disabled:opacity-50"
        >
          <i className="fa-solid fa-flag-checkered"></i>
          <span>{optionBLabel}</span>
        </button>
        <button
          onClick={() => decide('intervene')}
          disabled={busy}
          className="px-3.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold flex items-center gap-1.5 transition disabled:opacity-50"
        >
          <i className="fa-solid fa-terminal"></i>
          <span>{t('workspace.escalation.customCvo')}</span>
        </button>
      </div>
    </div>
  );
}
