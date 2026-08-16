import { useAppStore } from '../../../store/useAppStore';
import { useThreads } from '../../../hooks/useThreads';

export function PlanTab() {
  const { threads } = useThreads();
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const activeThread = threads.find((t) => t.id === activeThreadId);
  const activeTaskPlanSteps = activeThread?.taskPlanSteps ?? [];
  const hasPlan = activeTaskPlanSteps.length > 0;
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-slate-400">
        <span className="font-bold text-[11px] uppercase tracking-wider">Dynamic TaskPlan</span>
        <span className={`font-mono text-[10px] ${hasPlan ? 'text-emerald-400' : 'text-slate-500'}`}>
          {hasPlan ? 'Status: Executing' : 'Status: 无任务计划'}
        </span>
      </div>
      {hasPlan ? (
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
      ) : (
        <div className="text-[11px] text-slate-500 py-3 px-2 border border-dashed border-slate-700/60 rounded-xl bg-slate-950/40">
          当前线程暂无任务计划。后端尚未产出 taskPlanSteps（该能力为规划中特性，待 SOP/计划阶段后端实现后自动填充）。
        </div>
      )}
    </div>
  );
}
