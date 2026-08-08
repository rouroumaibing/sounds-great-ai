import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { SETTINGS_SECTIONS } from './settings-nav-config';

export function SettingsNav() {
  const activeSettingsTab = useAppStore((s) => s.activeSettingsTab);
  const setActiveSettingsTab = useAppStore((s) => s.setActiveSettingsTab);

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="p-3 border-b border-slate-800/80 flex items-center justify-between">
        <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
          <i className="fa-solid fa-gear text-indigo-400"></i>
          设置 (Settings)
        </span>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1 text-xs">
        {SETTINGS_SECTIONS.map((item) => (
          <button
            key={item.id}
            onClick={() => setActiveSettingsTab(item.id)}
            className={clsx(
              'w-full text-left px-3 py-2 rounded-xl flex items-center space-x-2.5 transition font-medium',
              activeSettingsTab === item.id
                ? 'bg-amber-600/20 text-amber-300 border border-amber-500/40 font-semibold shadow-sm'
                : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
            )}
          >
            <i className={clsx(item.icon, 'text-xs w-4 text-center', activeSettingsTab === item.id ? 'text-amber-400' : 'text-slate-500')}></i>
            <span>{item.label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
