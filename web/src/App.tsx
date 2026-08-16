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
import { ConnectionStatusBar } from './components/workspace/ConnectionStatusBar';
import { SettingsContent } from './components/settings/SettingsContent';
import { ProfilesContent } from './components/profiles/ProfilesContent';
import { PeopleMemoryContent } from './components/people-memory/PeopleMemoryContent';

const AboutPanel = lazy(() => import('./components/settings/AboutPanel').then(m => ({ default: m.AboutPanel })));
import { useAppStore } from './store/useAppStore';
import { useChatStore } from './store/useChatStore';

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
                {activeNav === 'settings' ? (
                  <SettingsContent activeSection={activeSettingsTab} />
                ) : activeNav === 'profiles' ? (
                  <ProfilesContent />
                ) : activeNav === 'people' ? (
                  <PeopleMemoryContent />
                ) : activeNav === 'about' ? (
                  <Suspense fallback={<div className="flex items-center justify-center h-full"><div className="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin" /></div>}>
                    <AboutPanel />
                  </Suspense>
                ) : (
                  <>
                    <ConnectionStatusBar />
                    <StreamTimeline />
                  </>
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
