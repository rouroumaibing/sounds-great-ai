import { useAppStore } from '../../store/useAppStore';
import { ThreadList } from '../threads/ThreadList';
import { SettingsNav } from '../settings/SettingsNav';
import { MemoryTab } from '../drawer/tabs/MemoryTab';
import { CustodyTrail } from '../workspace/CustodyTrail';

export function SecondaryPanel() {
  const activeNav = useAppStore((s) => s.activeNav);
  const activeThreadId = useAppStore((s) => s.activeThreadId);

  return (
    <div className="w-64 bg-slate-900/60 flex flex-col border-r border-slate-800/60 overflow-hidden">
      {activeNav === 'threads' && <ThreadList />}

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

      {activeNav === 'custody' && activeThreadId && <CustodyTrail threadId={activeThreadId} />}
    </div>
  );
}
