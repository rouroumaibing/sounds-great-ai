import { useState, useEffect } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { filterSections, useSettingsSections, type SettingsSection } from './settings-nav-config';

const PIN_STORAGE_KEY = 'sounds-great-ai:pinned-settings';

function usePinnedSections() {
  const [pinned, setPinned] = useState<Set<string>>(() => {
    try {
      const stored = localStorage.getItem(PIN_STORAGE_KEY);
      return new Set(stored ? JSON.parse(stored) : []);
    } catch {
      return new Set();
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(PIN_STORAGE_KEY, JSON.stringify([...pinned]));
    } catch {}
  }, [pinned]);

  const toggle = (id: string) => {
    setPinned((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return { pinned, toggle };
}

function NavItem({
  section,
  active,
  pinned,
  onPin,
  onSelect,
}: {
  section: SettingsSection;
  active: boolean;
  pinned: boolean;
  onPin: () => void;
  onSelect: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="group relative flex items-center">
      <button
        onClick={onSelect}
        data-guide-id={`settings.${section.id}`}
        className={clsx(
          'w-full text-left px-3 py-2 rounded-xl flex items-center space-x-2.5 transition font-medium',
          active
            ? 'bg-amber-600/20 text-amber-300 border border-amber-500/40 font-semibold shadow-sm'
            : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
        )}
      >
        <i className={clsx(section.icon, 'text-xs w-4 text-center', active ? 'text-amber-400' : 'text-slate-500')}></i>
        <span className="flex-1 truncate">{section.label}</span>
      </button>
      <button
        onClick={(e) => { e.stopPropagation(); onPin(); }}
        className={clsx(
          'absolute right-1 h-6 w-6 flex items-center justify-center rounded transition-opacity',
          pinned
            ? 'opacity-80 text-amber-400'
            : 'opacity-0 group-hover:opacity-60 text-slate-500 hover:text-slate-300'
        )}
        title={pinned ? t('common.unpin') : t('common.pinToTop')}
      >
        <i className={clsx('fa-solid fa-thumbtack text-[10px]', pinned && 'rotate-12')}></i>
      </button>
    </div>
  );
}

export function SettingsNav() {
  const { t } = useI18n();
  const activeSettingsTab = useAppStore((s) => s.activeSettingsTab);
  const setActiveSettingsTab = useAppStore((s) => s.setActiveSettingsTab);
  const [searchQuery, setSearchQuery] = useState('');
  const { pinned, toggle } = usePinnedSections();

  const sections = useSettingsSections();
  const filtered = filterSections(searchQuery, sections);
  const pinnedSections = filtered.filter((s) => pinned.has(s.id));
  const unpinnedSections = filtered.filter((s) => !pinned.has(s.id));

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="p-3 border-b border-slate-800/80 flex items-center justify-between">
        <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
          <i className="fa-solid fa-gear text-indigo-400"></i>
          {t('settingsNav.title')}
        </span>
      </div>

      <div className="px-2 py-2 border-b border-slate-800/60">
        <div className="relative">
          <i className="fa-solid fa-magnifying-glass absolute left-2.5 top-1/2 -translate-y-1/2 text-[10px] text-slate-500"></i>
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder={t('common.searchSettings')}
            className="w-full text-[11px] bg-slate-800/50 border border-slate-700/50 rounded-lg pl-7 pr-2 py-1.5 text-slate-300 placeholder-slate-500 focus:border-indigo-500/50 focus:outline-none transition"
          />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-1 text-xs">
        {filtered.length === 0 && searchQuery ? (
          <p className="text-center text-slate-500 py-4 text-[11px]">{t('common.noMatch')}</p>
        ) : (
          <>
            {pinnedSections.length > 0 && (
              <>
                {pinnedSections.map((section) => (
                  <NavItem
                    key={section.id}
                    section={section}
                    active={activeSettingsTab === section.id}
                    pinned={true}
                    onPin={() => toggle(section.id)}
                    onSelect={() => setActiveSettingsTab(section.id)}
                  />
                ))}
                {unpinnedSections.length > 0 && <div className="h-px bg-slate-800/40 my-1.5" />}
              </>
            )}
            {unpinnedSections.map((section) => (
              <NavItem
                key={section.id}
                section={section}
                active={activeSettingsTab === section.id}
                pinned={false}
                onPin={() => toggle(section.id)}
                onSelect={() => setActiveSettingsTab(section.id)}
              />
            ))}
          </>
        )}
      </div>
    </div>
  );
}
