import { useAppStore } from '../../../store/useAppStore';
import { useThreads } from '../../../hooks/useThreads';

export function PlanTab() {
  const { threads } = useThreads();
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const activeThread = threads.find((t) => t.id === activeThreadId);
  const activeTaskPlanSteps = activeThread?.taskPlanSteps ?? [];
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-slate-400">
        <span className="font-bold text-[11px] uppercase tracking-wider">Dynamic TaskPlan</span>
        <span className="font-mono text-[10px] text-emerald-400">Status: Executing</span>
      </div>
      <div className="space-y-2">
        {activeTaskPlanSteps.map((step, sIdx) => (
          <div key={sIdx} className={`p-2.5 rounded-xl border bg-slate-950/80 space-y-1.5 ${step.borderClass}`}>
            <div className="flex items-center justify-between">
              <span className="font-mono font-bold text-[11px] text-slate-200">Step {sIdx + 1}: {step.title}</span>
              <span className={`px-1.5 py-0.5 rounded text-[9px] font-mono border ${step.badgeClass}`}>{step.status}</span>
            </div>
            <p className="text-[11px] text-slate-400">{step.desc}</p>
            <div className="flex items-center justify-between text-[10px] text-slate-500 font-mono pt-1 border-t border-slate-800/60">
              <span>Assigned: {step.assignee}</span><span>{step.rule}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
