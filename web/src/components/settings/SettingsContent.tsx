import { lazy, Suspense } from 'react';
import { useSettingsSections } from './settings-nav-config';
import { SettingsPageHeader } from './primitives';

const MemberManagement = lazy(() => import('./MemberManagement').then(m => ({ default: m.MemberManagement })));
const AccountKeys = lazy(() => import('./AccountKeys').then(m => ({ default: m.AccountKeys })));

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
      <SettingsPageHeader title={meta.label} subtitle={meta.description} />
      <Suspense fallback={<LoadingSpinner />}>
        {Component && <Component />}
      </Suspense>
    </div>
  );
}
