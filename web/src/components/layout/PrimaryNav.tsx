import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useChatStore } from '../../store/useChatStore';
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
  // G4: any thread with an unresolved CVO escalation lights the badge.
  const hasUnreadEscalation = useChatStore((s) => Object.values(s.escalations).some(Boolean));
  const { t } = useI18n();

  return (
    <nav className="w-14 bg-slate-950 border-r border-slate-800/80 flex flex-col items-center py-3 justify-between shrink-0 select-none">
      {/* Top Navigation Icons */}
      <div className="flex flex-col items-center space-y-3 w-full px-2">
        <NavButton nav="threads" icon="fa-solid fa-comments" label={t('nav.threads')} activeNav={activeNav} onClick={() => setActiveNav('threads')} badge={hasUnreadEscalation} />
        <NavButton nav="memory" icon="fa-solid fa-database" label={t('nav.memory')} activeNav={activeNav} onClick={() => setActiveNav('memory')} />
        <NavButton nav="custody" icon="fa-solid fa-circle-nodes" label={t('nav.custody', '球权轨迹')} activeNav={activeNav} onClick={() => setActiveNav('custody')} />
        <NavButton nav="profiles" icon="fa-solid fa-paw" label={t('nav.profiles', '养熟')} activeNav={activeNav} onClick={() => setActiveNav('profiles')} />
        <NavButton nav="people" icon="fa-solid fa-users" label={t('nav.people', '人物记忆')} activeNav={activeNav} onClick={() => setActiveNav('people')} />
      </div>

      {/* Bottom Nav Icons */}
      <div className="flex flex-col items-center space-y-3 w-full px-2">
        <NavButton nav="about" icon="fa-solid fa-circle-info" label={t('nav.about')} activeNav={activeNav} onClick={() => setActiveNav('about')} />
        <NavButton nav="settings" icon="fa-solid fa-sliders" label={t('nav.settings')} activeNav={activeNav} onClick={() => setActiveNav('settings')} />
      </div>
    </nav>
  );
}
