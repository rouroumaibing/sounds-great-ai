import { useI18n } from '../../store/useI18n';
import type { SettingsNavItem, SettingsTabType } from '../../types';

export interface SettingsSection extends SettingsNavItem {
  description: string;
  color: string;
}

interface RawSection {
  id: SettingsTabType;
  labelKey: string;
  descKey: string;
  icon: string;
  color: string;
}

const RAW_SECTIONS: RawSection[] = [
  { id: 'members', labelKey: 'settings.members', descKey: 'settings.members.desc', icon: 'fa-solid fa-users-gear', color: 'indigo' },
  { id: 'personas', labelKey: 'settings.personas', descKey: 'settings.personas.desc', icon: 'fa-solid fa-id-card', color: 'indigo' },
  { id: 'accounts', labelKey: 'settings.accounts', descKey: 'settings.accounts.desc', icon: 'fa-solid fa-key', color: 'indigo' },
  { id: 'config', labelKey: 'settings.config', descKey: 'settings.config.desc', icon: 'fa-solid fa-sliders', color: 'amber' },
  { id: 'rules', labelKey: 'settings.rules', descKey: 'settings.rules.desc', icon: 'fa-solid fa-book', color: 'amber' },
  { id: 'skills', labelKey: 'settings.skills', descKey: 'settings.skills.desc', icon: 'fa-solid fa-puzzle-piece', color: 'amber' },
  { id: 'mcp', labelKey: 'settings.mcp', descKey: 'settings.mcp.desc', icon: 'fa-solid fa-plug', color: 'amber' },
  { id: 'notifications', labelKey: 'settings.notifications', descKey: 'settings.notifications.desc', icon: 'fa-solid fa-bell', color: 'amber' },
  { id: 'im', labelKey: 'settings.im', descKey: 'settings.im.desc', icon: 'fa-solid fa-comments', color: 'amber' },
  { id: 'plugins', labelKey: 'settings.plugins', descKey: 'settings.plugins.desc', icon: 'fa-solid fa-puzzle-piece', color: 'amber' },
  { id: 'marketplace', labelKey: 'settings.marketplace', descKey: 'settings.marketplace.desc', icon: 'fa-solid fa-store', color: 'amber' },
  { id: 'concierge', labelKey: 'settings.concierge', descKey: 'settings.concierge.desc', icon: 'fa-solid fa-circle-dot', color: 'emerald' },
  { id: 'voice', labelKey: 'settings.voice', descKey: 'settings.voice.desc', icon: 'fa-solid fa-microphone', color: 'emerald' },
  { id: 'ops', labelKey: 'settings.ops', descKey: 'settings.ops.desc', icon: 'fa-solid fa-heart-pulse', color: 'emerald' },
  { id: 'eval', labelKey: 'settings.eval', descKey: 'settings.eval.desc', icon: 'fa-solid fa-clipboard-check', color: 'emerald' },
  { id: 'about', labelKey: 'settings.about', descKey: 'settings.about.desc', icon: 'fa-solid fa-circle-info', color: 'slate' },
];

export const DEFAULT_SECTION = 'members';

const _t = useI18n.getState().t.bind(useI18n.getState());

const SECTION_KEYWORDS: Record<string, string> = {
  members: _t('settings.settings-nav-config.s1'),
  personas: _t('settings.settings-nav-config.s2'),
  accounts: _t('settings.settings-nav-config.s3'),
  config: _t('settings.settings-nav-config.s4'),
  rules: _t('settings.settings-nav-config.s5'),
  skills: _t('settings.settings-nav-config.s6'),
  mcp: _t('settings.settings-nav-config.s7'),
  notifications: _t('settings.settings-nav-config.s8'),
  im: _t('settings.settings-nav-config.s9'),
  plugins: _t('settings.settings-nav-config.s10'),
  marketplace: _t('settings.settings-nav-config.s11'),
  concierge: _t('settings.settings-nav-config.s12'),
  voice: _t('settings.settings-nav-config.s13'),
  ops: _t('settings.settings-nav-config.s14'),
  eval: _t('settings.settings-nav-config.s15'),
  about: _t('settings.settings-nav-config.s16'),
};

export function useSettingsSections(): SettingsSection[] {
  const { t } = useI18n();
  return RAW_SECTIONS.map((s) => ({
    id: s.id,
    label: t(s.labelKey),
    icon: s.icon,
    color: s.color,
    description: t(s.descKey),
  }));
}

export function filterSections(query: string, sections: SettingsSection[]): SettingsSection[] {
  const q = query.toLowerCase().trim();
  if (!q) return sections;
  return sections.filter(
    (s) =>
      s.label.toLowerCase().includes(q) ||
      s.id.toLowerCase().includes(q) ||
      (SECTION_KEYWORDS[s.id] ?? '').toLowerCase().includes(q),
  );
}
