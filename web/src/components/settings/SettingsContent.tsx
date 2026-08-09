import { lazy, Suspense } from 'react';
import { useSettingsSections } from './settings-nav-config';

const MemberManagement = lazy(() => import('./MemberManagement').then(m => ({ default: m.MemberManagement })));
const AccountKeys = lazy(() => import('./AccountKeys').then(m => ({ default: m.AccountKeys })));
const PersonasPanel = lazy(() => import('./PersonasPanel').then(m => ({ default: m.PersonasPanel })));
const SkillsPanel = lazy(() => import('./SkillsPanel').then(m => ({ default: m.SkillsPanel })));
const McpPanel = lazy(() => import('./McpPanel').then(m => ({ default: m.McpPanel })));
const ConfigPanel = lazy(() => import('./ConfigPanel').then(m => ({ default: m.ConfigPanel })));
const RulesPanel = lazy(() => import('./RulesPanel').then(m => ({ default: m.RulesPanel })));
const NotificationsPanel = lazy(() => import('./NotificationsPanel').then(m => ({ default: m.NotificationsPanel })));
const AboutPanel = lazy(() => import('./AboutPanel').then(m => ({ default: m.AboutPanel })));
const OpsPanel = lazy(() => import('./OpsPanel').then(m => ({ default: m.OpsPanel })));
const EvalPanel = lazy(() => import('./EvalPanel').then(m => ({ default: m.EvalPanel })));
const ImPanel = lazy(() => import('./ImPanel').then(m => ({ default: m.ImPanel })));
const PluginsPanel = lazy(() => import('./PluginsPanel').then(m => ({ default: m.PluginsPanel })));
const MarketplacePanel = lazy(() => import('./MarketplacePanel').then(m => ({ default: m.MarketplacePanel })));
const ConciergePanel = lazy(() => import('./ConciergePanel').then(m => ({ default: m.ConciergePanel })));
const VoicePanel = lazy(() => import('./VoicePanel').then(m => ({ default: m.VoicePanel })));

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center h-full">
      <div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
    </div>
  );
}

const SECTION_COMPONENTS: Record<string, React.LazyExoticComponent<React.ComponentType>> = {
  members: MemberManagement,
  accounts: AccountKeys,
  personas: PersonasPanel,
  skills: SkillsPanel,
  mcp: McpPanel,
  config: ConfigPanel,
  rules: RulesPanel,
  notifications: NotificationsPanel,
  im: ImPanel,
  plugins: PluginsPanel,
  marketplace: MarketplacePanel,
  concierge: ConciergePanel,
  voice: VoicePanel,
  about: AboutPanel,
  ops: OpsPanel,
  eval: EvalPanel,
};

interface SettingsContentProps {
  activeSection: string;
}

export function SettingsContent({ activeSection }: SettingsContentProps) {
  const sections = useSettingsSections();
  const meta = sections.find((s) => s.id === activeSection) ?? sections[0];
  const Component = SECTION_COMPONENTS[meta.id];

  return (
    <div className="flex-1 flex flex-col bg-slate-950 overflow-y-auto p-6 md:p-8 space-y-6">
      <div className="border-b border-slate-800/80 pb-4">
        <h2 className="text-xl font-bold text-slate-100">{meta.label}</h2>
        <p className="text-xs text-slate-400 mt-1">{meta.description}</p>
      </div>
      <Suspense fallback={<LoadingSpinner />}>
        {Component && <Component />}
      </Suspense>
    </div>
  );
}
