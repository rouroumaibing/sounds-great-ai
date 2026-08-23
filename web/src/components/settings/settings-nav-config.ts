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

// Retained settings sections. Concierge / voice / connectors landed with
// panels-roadmap P1+P2 (docs/plans/panels-roadmap.md); plugins / marketplace
// stay out of the nav until P3/P4.
const RAW_SECTIONS: RawSection[] = [
  { id: 'accounts', label: '账户与密钥', desc: '模型账户、凭据和执行身份的归属关系。', icon: 'fa-solid fa-key', color: 'indigo' },
  { id: 'members', label: '成员管理', desc: '狗狗名册、默认协作对象与编排顺序。', icon: 'fa-solid fa-users-gear', color: 'indigo' },
  { id: 'dossier', label: '狗狗画像', desc: '能力画像档案、观察暂存与蒸馏提案审批。', icon: 'fa-solid fa-id-card', color: 'indigo' },
  { id: 'skills', label: '技能管理', desc: '技能启用、carrier 挂载与漂移治理。', icon: 'fa-solid fa-wand-magic-sparkles', color: 'indigo' },
  { id: 'plugins', label: '插件管理', desc: '插件包安装（zip）、安全审查门禁与启停。', icon: 'fa-solid fa-puzzle-piece', color: 'indigo' },
  { id: 'concierge', label: '迎宾犬', desc: '悬浮球外观、人设与主动策略。', icon: 'fa-solid fa-dog', color: 'indigo' },
  { id: 'voice', label: '语音配置', desc: 'TTS/STT 参数与发音词汇表。', icon: 'fa-solid fa-volume-high', color: 'indigo' },
  { id: 'im', label: 'IM 连接器', desc: '外部 IM/webhook 连接器注册与探活。', icon: 'fa-solid fa-plug', color: 'indigo' },
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
