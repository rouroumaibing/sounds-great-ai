import { lazy, Suspense, useEffect } from 'react';
import { Group, Panel, Separator, usePanelRef } from 'react-resizable-panels';
import { Header } from './components/layout/Header';
import { PrimaryNav } from './components/layout/PrimaryNav';
import { SecondaryPanel } from './components/layout/SecondaryPanel';
import { ToolPanel } from './components/drawer/ToolPanel';
import { CatHueInjector } from './components/CatHueInjector';
import { ContextMenu } from './components/common/ContextMenu';
import { ToastContainer } from './components/common/ToastContainer';
import { StreamTimeline } from './components/workspace/StreamTimeline';
import { useAppStore } from './store/useAppStore';
import { useChatStore } from './store/useChatStore';

// Lazy-loaded settings panels (code-splitting)
const MemberManagement = lazy(() => import('./components/settings/MemberManagement').then(m => ({ default: m.MemberManagement })));
const AccountKeys = lazy(() => import('./components/settings/AccountKeys').then(m => ({ default: m.AccountKeys })));
const PersonasPanel = lazy(() => import('./components/settings/PersonasPanel').then(m => ({ default: m.PersonasPanel })));
const SkillsPanel = lazy(() => import('./components/settings/SkillsPanel').then(m => ({ default: m.SkillsPanel })));
const McpPanel = lazy(() => import('./components/settings/McpPanel').then(m => ({ default: m.McpPanel })));
const ConfigPanel = lazy(() => import('./components/settings/ConfigPanel').then(m => ({ default: m.ConfigPanel })));
const RulesPanel = lazy(() => import('./components/settings/RulesPanel').then(m => ({ default: m.RulesPanel })));
const NotificationsPanel = lazy(() => import('./components/settings/NotificationsPanel').then(m => ({ default: m.NotificationsPanel })));
const AboutPanel = lazy(() => import('./components/settings/AboutPanel').then(m => ({ default: m.AboutPanel })));
const OpsPanel = lazy(() => import('./components/settings/OpsPanel').then(m => ({ default: m.OpsPanel })));
const EvalPanel = lazy(() => import('./components/settings/EvalPanel').then(m => ({ default: m.EvalPanel })));
const ImPanel = lazy(() => import('./components/settings/ImPanel').then(m => ({ default: m.ImPanel })));
const PluginsPanel = lazy(() => import('./components/settings/PluginsPanel').then(m => ({ default: m.PluginsPanel })));
const MarketplacePanel = lazy(() => import('./components/settings/MarketplacePanel').then(m => ({ default: m.MarketplacePanel })));
const ConciergePanel = lazy(() => import('./components/settings/ConciergePanel').then(m => ({ default: m.ConciergePanel })));
const VoicePanel = lazy(() => import('./components/settings/VoicePanel').then(m => ({ default: m.VoicePanel })));

function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center h-full">
      <div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" />
    </div>
  );
}

function App() {
  const leftPanelVisible = useAppStore((s) => s.leftPanelVisible);
  const middlePanelVisible = useAppStore((s) => s.middlePanelVisible);
  const rightPanelVisible = useAppStore((s) => s.rightPanelVisible);
  const activeNav = useAppStore((s) => s.activeNav);
  const activeSettingsTab = useAppStore((s) => s.activeSettingsTab);

  const initWebSocket = useChatStore((s) => s.initWebSocket);
  const fetchNotifications = useAppStore((s) => s.fetchNotifications);
  const fetchFileTree = useAppStore((s) => s.fetchFileTree);

  const leftPanelRef = usePanelRef();
  const middlePanelRef = usePanelRef();
  const rightPanelRef = usePanelRef();

  useEffect(() => {
    initWebSocket();
  }, [initWebSocket]);

  useEffect(() => {
    fetchNotifications();
    fetchFileTree();
  }, [fetchNotifications, fetchFileTree]);

  useEffect(() => {
    if (leftPanelVisible) leftPanelRef.current?.expand();
    else leftPanelRef.current?.collapse();
  }, [leftPanelVisible]);

  useEffect(() => {
    if (middlePanelVisible) middlePanelRef.current?.expand();
    else middlePanelRef.current?.collapse();
  }, [middlePanelVisible]);

  useEffect(() => {
    if (rightPanelVisible) rightPanelRef.current?.expand();
    else rightPanelRef.current?.collapse();
  }, [rightPanelVisible]);

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-slate-950">
      <CatHueInjector />
      <Header />
      <div className="flex-1 min-h-0 relative">
        <Group orientation="horizontal" className="h-full">
          <Panel panelRef={leftPanelRef} id="left-panel" collapsible collapsedSize="0%" minSize="15%" defaultSize="20%" maxSize="30%" className="h-full">
            <div className="h-full flex flex-col min-h-0 overflow-hidden border-r border-slate-800/80 bg-slate-950">
              <aside className="flex-1 flex min-h-0 overflow-hidden">
                <PrimaryNav />
                <SecondaryPanel />
              </aside>
            </div>
          </Panel>
          <Separator className="w-px bg-slate-800 hover:bg-indigo-500 transition-colors" />
          <Panel panelRef={middlePanelRef} id="middle-panel" collapsible collapsedSize="0%" minSize="25%" defaultSize="35%" maxSize="50%" className="h-full">
            <div className="h-full flex flex-col min-h-0 overflow-hidden bg-slate-950">
              <main className="flex-1 flex flex-col min-h-0 overflow-hidden relative">
                {activeNav !== 'settings' ? (
                  <StreamTimeline />
                ) : (
                  <div className="flex-1 flex flex-col bg-slate-950 overflow-y-auto p-6 md:p-8 space-y-6">
                    <Suspense fallback={<LoadingSpinner />}>
                      {activeSettingsTab === 'members' && <MemberManagement />}
                      {activeSettingsTab === 'accounts' && <AccountKeys />}
                      {activeSettingsTab === 'personas' && <PersonasPanel />}
                      {activeSettingsTab === 'skills' && <SkillsPanel />}
                      {activeSettingsTab === 'mcp' && <McpPanel />}
                      {activeSettingsTab === 'config' && <ConfigPanel />}
                      {activeSettingsTab === 'rules' && <RulesPanel />}
                      {activeSettingsTab === 'notifications' && <NotificationsPanel />}
                      {activeSettingsTab === 'im' && <ImPanel />}
                      {activeSettingsTab === 'plugins' && <PluginsPanel />}
                      {activeSettingsTab === 'marketplace' && <MarketplacePanel />}
                      {activeSettingsTab === 'concierge' && <ConciergePanel />}
                      {activeSettingsTab === 'voice' && <VoicePanel />}
                      {activeSettingsTab === 'about' && <AboutPanel />}
                      {activeSettingsTab === 'ops' && <OpsPanel />}
                      {activeSettingsTab === 'eval' && <EvalPanel />}
                    </Suspense>
                  </div>
                )}
              </main>
            </div>
          </Panel>
          <Separator className="w-px bg-slate-800 hover:bg-indigo-500 transition-colors" />
          <Panel panelRef={rightPanelRef} id="right-panel" collapsible collapsedSize="0%" minSize="30%" defaultSize="45%" maxSize="60%" className="h-full">
            <div className="h-full flex flex-col min-h-0 overflow-hidden">
              <ToolPanel />
            </div>
          </Panel>
        </Group>
      </div>
      <ContextMenu />
      <ToastContainer />
    </div>
  );
}

export default App;
