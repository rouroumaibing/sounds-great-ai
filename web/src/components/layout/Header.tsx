import clsx from 'clsx';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { useThreads } from '../../hooks/useThreads';

export function Header() {
  const activeNav = useAppStore((s) => s.activeNav);
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const toggleLeftPanel = useAppStore((s) => s.toggleLeftPanel);
  const toggleMiddlePanel = useAppStore((s) => s.toggleMiddlePanel);
  const toggleRightPanel = useAppStore((s) => s.toggleRightPanel);
  const leftPanelVisible = useAppStore((s) => s.leftPanelVisible);
  const middlePanelVisible = useAppStore((s) => s.middlePanelVisible);
  const rightPanelVisible = useAppStore((s) => s.rightPanelVisible);
  const wsReadyState = useChatStore((s) => s.wsReadyState);
  const wsReady = wsReadyState === 1; // WebSocket.OPEN
  const { threads } = useThreads();

  const activeThread = threads.find((t) => t.id === activeThreadId);
  const activeTitle = activeNav === 'settings'
    ? '系统配置'
    : activeThread?.title ?? '';

  return (
    <header className="h-14 border-b border-slate-800/80 bg-slate-900/90 backdrop-blur-md flex items-center justify-between px-4 z-20 shrink-0">
      {/* Left Logo & App Info */}
      <div className="flex items-center space-x-3">
        <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-indigo-500 via-purple-600 to-rose-500 flex items-center justify-center text-white font-bold shadow-lg shadow-indigo-500/20">
          <i className="fa-solid fa-dog text-lg"></i>
        </div>
        <div>
          <div className="flex items-center space-x-2">
            <span className="font-extrabold text-slate-100 tracking-wide text-base">sounds-great-ai</span>
            <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">Sounds Great AI</span>
          </div>
          <p className="text-[11px] text-slate-400">Dog Pack Multi-Agent Command Deck</p>
        </div>
      </div>

      {/* Center Active Task Status Indicator */}
      <div className="flex items-center space-x-4 bg-slate-950/80 border border-slate-800 rounded-xl px-4 py-1.5 shadow-inner">
        <div className="flex items-center space-x-2 text-xs">
          <span className="text-slate-500 font-mono">Active Module:</span>
          <span className="font-mono text-slate-200 font-medium truncate max-w-md">
            {activeTitle}
          </span>
        </div>
        <div className="h-3 w-px bg-slate-800"></div>
        <div className="flex items-center space-x-2 text-xs">
          <span className="relative flex h-2 w-2">
            <span className={clsx('animate-ping absolute inline-flex h-full w-full rounded-full opacity-75', wsReady ? 'bg-emerald-400' : 'bg-rose-400')}></span>
            <span className={clsx('relative inline-flex rounded-full h-2 w-2', wsReady ? 'bg-emerald-500' : 'bg-rose-500')}></span>
          </span>
          <span className={clsx('font-mono text-[11px] font-semibold', wsReady ? 'text-emerald-400' : 'text-rose-400')}>
            {wsReady ? 'WS CONNECTED' : 'WS DISCONNECTED'}
          </span>
        </div>
      </div>

      {/* Right Actions & Panel Toggles */}
      <div className="flex items-center space-x-2.5">
        <div className="flex items-center space-x-1 border-l border-slate-800 pl-2.5">
          <button onClick={toggleLeftPanel} className={clsx('p-2 rounded-lg border text-xs transition', leftPanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title="切换左侧面板">
            <i className="fa-solid fa-table-columns"></i>
          </button>
          <button onClick={toggleMiddlePanel} className={clsx('p-2 rounded-lg border text-xs transition', middlePanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title="切换中间面板">
            <i className="fa-solid fa-table-list"></i>
          </button>
          <button onClick={toggleRightPanel} className={clsx('p-2 rounded-lg border text-xs transition', rightPanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title="切换右侧工具面板">
            <i className="fa-solid fa-window-maximize"></i>
          </button>
        </div>
      </div>
    </header>
  );
}
