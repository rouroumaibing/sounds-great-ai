import { useAppStore } from '../../store/useAppStore';
import { useThreads } from '../../hooks/useThreads';
import { ThreadList } from '../threads/ThreadList';
import { DogPackGrid } from '../agents/DogPackGrid';
import { SettingsNav } from '../settings/SettingsNav';
import { MemoryTab } from '../drawer/tabs/MemoryTab';

export function SecondaryPanel() {
  const activeNav = useAppStore((s) => s.activeNav);
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const { threads } = useThreads();

  const activeThread = threads.find((t) => t.id === activeThreadId);
  const activeTaskPlanSteps = activeThread?.taskPlanSteps ?? [];

  return (
    <div className="w-64 bg-slate-900/60 flex flex-col border-r border-slate-800/60 overflow-hidden">
      {activeNav === 'threads' && <ThreadList />}

      {activeNav === 'agents' && <DogPackGrid />}

      {activeNav === 'tasks' && (
        <div className="flex-1 flex flex-col p-3 overflow-y-auto space-y-3">
          <div className="flex items-center justify-between border-b border-slate-800/80 pb-2">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
              <i className="fa-solid fa-diagram-project text-indigo-400"></i>
              TaskPlan & SOP
            </span>
          </div>
          <div className="space-y-2 text-xs">
            {activeTaskPlanSteps.map((step, sIdx) => (
              <div key={sIdx} className={`p-2.5 rounded-xl border bg-slate-950/60 space-y-1 ${step.borderClass}`}>
                <div className="flex items-center justify-between">
                  <span className="font-mono font-bold text-[11px] text-slate-200">Step {sIdx + 1}</span>
                  <span className={`px-1.5 py-0.5 rounded text-[9px] font-mono border ${step.badgeClass}`}>
                    {step.status}
                  </span>
                </div>
                <div className="text-slate-200 font-medium text-[11px]">{step.title}</div>
                <div className="text-[10px] text-slate-400 font-mono pt-1 flex justify-between">
                  <span>@{step.assignee}</span>
                  <span className="text-slate-500">{step.rule}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {activeNav === 'memory' && (
        <div className="flex-1 flex flex-col p-3 overflow-y-auto space-y-3">
          <div className="flex items-center justify-between border-b border-slate-800/80 pb-2">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
              <i className="fa-solid fa-database text-indigo-400"></i>
              Evidence Store
            </span>
            <span className="text-[10px] font-mono text-slate-500">SQLite</span>
          </div>
          <MemoryTab />
        </div>
      )}

      {activeNav === 'settings' && <SettingsNav />}
    </div>
  );
}
