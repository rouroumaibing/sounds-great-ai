import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useThreads } from '../../hooks/useThreads';
import type { PrimaryNavType } from '../../types';

interface NavButtonProps {
  nav: PrimaryNavType;
  icon: string;
  label: string;
  activeNav: PrimaryNavType;
  onClick: () => void;
  badge?: boolean;
}

function NavButton({ nav, icon, label, activeNav, onClick, badge }: NavButtonProps) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'w-10 h-10 rounded-xl flex items-center justify-center transition-all relative group',
        activeNav === nav
          ? 'bg-indigo-600 text-white shadow-lg shadow-indigo-600/30'
          : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'
      )}
    >
      <i className={clsx(icon, 'text-sm')}></i>
      {badge && (
        <span className="absolute top-1 right-1 w-2.5 h-2.5 rounded-full bg-rose-500 ring-2 ring-slate-950 animate-pulse"></span>
      )}
      <span className="absolute left-14 bg-slate-900 border border-slate-700 text-slate-200 text-[10px] px-2 py-1 rounded-md whitespace-nowrap hidden group-hover:block z-30 font-mono shadow-xl">
        {label}
      </span>
    </button>
  );
}

export function PrimaryNav() {
  const activeNav = useAppStore((s) => s.activeNav);
  const setActiveNav = useAppStore((s) => s.setActiveNav);
  const { threads } = useThreads();
  const hasUnreadEscalation = threads.some((t) => t.hasEscalation);

  return (
    <nav className="w-14 bg-slate-950 border-r border-slate-800/80 flex flex-col items-center py-3 justify-between shrink-0 select-none">
      {/* Top Navigation Icons */}
      <div className="flex flex-col items-center space-y-3 w-full px-2">
        <NavButton nav="threads" icon="fa-solid fa-comments" label="线程与对话 (Threads)" activeNav={activeNav} onClick={() => setActiveNav('threads')} badge={hasUnreadEscalation} />
        <NavButton nav="agents" icon="fa-solid fa-shield-dog" label="犬种特工队 (Dog Pack)" activeNav={activeNav} onClick={() => setActiveNav('agents')} />
        <NavButton nav="tasks" icon="fa-solid fa-diagram-project" label="任务编排 (TaskPlans)" activeNav={activeNav} onClick={() => setActiveNav('tasks')} />
        <NavButton nav="memory" icon="fa-solid fa-database" label="共享记忆 (Memory Evidence)" activeNav={activeNav} onClick={() => setActiveNav('memory')} />
      </div>

      {/* Bottom Nav Icons */}
      <div className="flex flex-col items-center space-y-3 w-full px-2">
        <NavButton nav="settings" icon="fa-solid fa-sliders" label="系统配置" activeNav={activeNav} onClick={() => setActiveNav('settings')} />
      </div>
    </nav>
  );
}
