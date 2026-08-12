import { useState } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useSettingsSections, type SettingsSection } from './settings-nav-config';

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

// NavItem: rounded-lg (8px) items, h-9,
// icon + label, active state tinted amber, with a hover-reveal pin button.
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
          'w-full text-left px-2.5 h-9 rounded-lg flex items-center space-x-2.5 transition font-medium',
          active
            ? 'bg-amber-600/20 text-amber-300 border border-amber-500/40 font-semibold shadow-sm'
            : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200',
        )}
      >
        <i className={clsx(section.icon, 'text-xs w-4 text-center', active ? 'text-amber-400' : 'text-slate-500')}></i>
        <span className="flex-1 truncate">{section.label}</span>
      </button>
      <button
        onClick={(e) => {
          e.stopPropagation();
          onPin();
        }}
        className={clsx(
          'absolute right-1 h-6 w-6 flex items-center justify-center rounded transition-opacity',
          pinned
            ? 'opacity-80 text-amber-400'
            : 'opacity-0 group-hover:opacity-60 text-slate-500 hover:text-slate-300',
        )}
        title={pinned ? t('common.unpin') : t('common.pinToTop')}
      >
        <i className={clsx('fa-solid fa-thumbtack text-[10px]', pinned && 'rotate-12')}></i>
      </button>
    </div>
  );
}

// Renders a flat, search-less list of sections (pinned state still
// tracked but not split into a separate group). Search lives elsewhere.
export function SettingsNav() {
  const { t } = useI18n();
  const activeSettingsTab = useAppStore((s) => s.activeSettingsTab);
  const setActiveSettingsTab = useAppStore((s) => s.setActiveSettingsTab);
  const { pinned, toggle } = usePinnedSections();
  const sections = useSettingsSections();

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div className="px-4 pt-4 pb-2">
        <h1 className="text-lg font-bold text-slate-100">{t('settingsNav.title')}</h1>
      </div>
      <div className="flex-1 overflow-y-auto p-2 space-y-0.5 text-xs">
        {sections.map((section) => (
          <NavItem
            key={section.id}
            section={section}
            active={activeSettingsTab === section.id}
            pinned={pinned.has(section.id)}
            onPin={() => toggle(section.id)}
            onSelect={() => setActiveSettingsTab(section.id)}
          />
        ))}
      </div>
    </div>
  );
}
