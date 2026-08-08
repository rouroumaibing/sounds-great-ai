import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { DRAWER_TABS } from './drawer-tabs';
import { PlanTab } from './tabs/PlanTab';
import { McpTab } from './tabs/McpTab';
import { MemoryTab } from './tabs/MemoryTab';
import { FilesTab } from './tabs/FilesTab';
import { SessionChainTab } from './tabs/SessionChainTab';

export function ToolPanel() {
  const activeDrawerTab = useAppStore((s) => s.activeDrawerTab);
  const setActiveDrawerTab = useAppStore((s) => s.setActiveDrawerTab);

  return (
    <aside className="h-full flex flex-col bg-slate-900/60 z-10">
      <div className="flex items-center border-b border-slate-800 bg-slate-950 flex-shrink-0">
        {DRAWER_TABS.map((tab) => (
          <button key={tab.id} onClick={() => setActiveDrawerTab(tab.id)}
            className={clsx('flex-1 py-2.5 text-center text-xs font-medium border-b-2 transition flex items-center justify-center gap-1.5 relative', activeDrawerTab === tab.id ? 'border-indigo-500 text-indigo-400 bg-slate-900/40' : 'border-transparent text-slate-400 hover:text-slate-200')}>
            <i className={tab.icon}></i><span>{tab.label}</span>
          </button>
        ))}
      </div>
      <div className="flex-1 min-h-0 overflow-y-auto p-3 space-y-4 text-xs">
        <div className={activeDrawerTab === 'plan' ? 'block' : 'hidden'}><PlanTab /></div>
        <div className={activeDrawerTab === 'mcp' ? 'block' : 'hidden'}><McpTab /></div>
        <div className={activeDrawerTab === 'memory' ? 'block' : 'hidden'}><MemoryTab /></div>
        <div className={activeDrawerTab === 'files' ? 'block' : 'hidden'}><FilesTab /></div>
        <div className={activeDrawerTab === 'session-chain' ? 'block' : 'hidden'}><SessionChainTab /></div>
      </div>
    </aside>
  );
}
