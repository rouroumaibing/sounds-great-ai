import type { SettingsNavItem, SettingsTabType } from '../../types';

export interface SettingsSection extends SettingsNavItem {
  description: string;
  color: string;
}

interface RawSection {
  id: SettingsTabType;
  label: string;
  desc: string;
  icon: string;
  color: string;
}

// Retained settings sections: Members roster and Account & Keys.
// (Other sections removed at the user's request — keep the settings app focused.
//  'about' was promoted to a top-level PrimaryNav item at the user's request.)
const RAW_SECTIONS: RawSection[] = [
  { id: 'accounts', label: '账户与密钥', desc: '模型账户、凭据和执行身份的归属关系。', icon: 'fa-solid fa-key', color: 'indigo' },
  { id: 'members', label: '成员管理', desc: '狗狗名册、默认协作对象与编排顺序。', icon: 'fa-solid fa-users-gear', color: 'indigo' },
  { id: 'system', label: '系统配置', desc: '环境选项、默认行为和运行时总开关。', icon: 'fa-solid fa-gear', color: 'indigo' },
  { id: 'ops', label: '运维监控', desc: '服务健康、运行态观测与评估。', icon: 'fa-solid fa-chart-line', color: 'indigo' },
];

export const DEFAULT_SECTION = 'accounts';

export function useSettingsSections(): SettingsSection[] {
  return RAW_SECTIONS.map((s) => ({
    id: s.id,
    label: s.label,
    icon: s.icon,
    color: s.color,
    description: s.desc,
  }));
}

export function filterSections(query: string, sections: SettingsSection[]): SettingsSection[] {
  const q = query.toLowerCase().trim();
  if (!q) return sections;
  return sections.filter((s) => s.label.toLowerCase().includes(q) || s.id.toLowerCase().includes(q));
}
