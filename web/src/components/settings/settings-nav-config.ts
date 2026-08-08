import type { SettingsNavItem } from '../../types';

export const SETTINGS_SECTIONS: SettingsNavItem[] = [
  { id: 'members', label: '成员管理', icon: 'fa-solid fa-users-gear' },
  { id: 'personas', label: '犬种画像', icon: 'fa-solid fa-id-card' },
  { id: 'accounts', label: '账户与密钥', icon: 'fa-solid fa-key' },
  { id: 'config', label: '系统配置', icon: 'fa-solid fa-sliders' },
  { id: 'rules', label: '协作与规则', icon: 'fa-solid fa-book' },
  { id: 'skills', label: '技能管理', icon: 'fa-solid fa-puzzle-piece' },
  { id: 'mcp', label: 'MCP 服务', icon: 'fa-solid fa-plug' },
  { id: 'notifications', label: '通知中心', icon: 'fa-solid fa-bell' },
  { id: 'im', label: 'IM 对接', icon: 'fa-solid fa-comments' },
  { id: 'plugins', label: '插件集成', icon: 'fa-solid fa-puzzle-piece' },
  { id: 'marketplace', label: '能力市场', icon: 'fa-solid fa-store' },
  { id: 'concierge', label: '犬种球', icon: 'fa-solid fa-circle-dot' },
  { id: 'voice', label: '语音管理', icon: 'fa-solid fa-microphone' },
  { id: 'ops', label: '运维监控', icon: 'fa-solid fa-heart-pulse' },
  { id: 'eval', label: '评估中心', icon: 'fa-solid fa-clipboard-check' },
  { id: 'about', label: '关于', icon: 'fa-solid fa-circle-info' },
];

export const DEFAULT_SECTION = 'members';
