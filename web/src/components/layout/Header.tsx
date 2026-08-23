import clsx from 'clsx';
import { useState } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { useThreads } from '../../hooks/useThreads';
import { useI18n } from '../../store/useI18n';
import { NotificationCenter } from '../common/NotificationCenter';

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
  const wsReady = wsReadyState === 1;
  const { threads } = useThreads();
  const { locale, setLocale, t } = useI18n();
  const [langOpen, setLangOpen] = useState(false);

  const activeThread = threads.find((t) => t.id === activeThreadId);
  const activeTitle = activeNav === 'settings'
    ? t('header.systemConfig')
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
            <span className="font-extrabold text-slate-100 tracking-wide text-base">{t('header.appName')}</span>
            <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">Sounds Great AI</span>
          </div>
          <p className="text-[11px] text-slate-400">{t('header.appSubtitle')}</p>
        </div>
      </div>

      {/* Center Active Task Status Indicator */}
      <div className="flex items-center space-x-4 bg-slate-950/80 border border-slate-800 rounded-xl px-4 py-1.5 shadow-inner">
        <div className="flex items-center space-x-2 text-xs">
          <span className="text-slate-500 font-mono">{t('header.activeModule')}:</span>
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
            {wsReady ? t('header.wsConnected') : t('header.wsDisconnected')}
          </span>
        </div>
      </div>

      {/* Right Actions & Panel Toggles */}
      <div className="flex items-center space-x-2.5">
        {/* Language Switcher */}
        <div className="relative">
          <button
            onClick={() => setLangOpen(!langOpen)}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-slate-800 text-slate-400 hover:text-slate-200 hover:border-slate-700 transition text-xs"
          >
            <i className="fa-solid fa-globe"></i>
            <span>{locale === 'zh-CN' ? '中文' : 'English'}</span>
            <i className={clsx('fa-solid fa-chevron-down text-[10px] transition-transform', langOpen && 'rotate-180')}></i>
          </button>
          {langOpen && (
            <>
              <div className="fixed inset-0 z-30" onClick={() => setLangOpen(false)} />
              <div className="absolute right-0 top-full mt-1 z-40 w-32 rounded-xl bg-slate-900 border border-slate-800 shadow-xl py-1">
                <button
                  onClick={() => { setLocale('zh-CN'); setLangOpen(false); }}
                  className={clsx('w-full px-3 py-2 text-left text-xs transition flex items-center gap-2', locale === 'zh-CN' ? 'text-indigo-400 bg-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60')}
                >
                  <i className="fa-solid fa-language text-[10px]"></i>中文
                </button>
                <button
                  onClick={() => { setLocale('en'); setLangOpen(false); }}
                  className={clsx('w-full px-3 py-2 text-left text-xs transition flex items-center gap-2', locale === 'en' ? 'text-indigo-400 bg-indigo-500/10' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60')}
                >
                  <i className="fa-solid fa-language text-[10px]"></i>English
                </button>
              </div>
            </>
          )}
        </div>

        <div className="flex items-center space-x-1 border-l border-slate-800 pl-2.5">
          <NotificationCenter />
          <button onClick={toggleLeftPanel} className={clsx('p-2 rounded-lg border text-xs transition', leftPanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title={t('header.toggleLeft')}>
            <i className="fa-solid fa-table-columns"></i>
          </button>
          <button onClick={toggleMiddlePanel} className={clsx('p-2 rounded-lg border text-xs transition', middlePanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title={t('header.toggleMiddle')}>
            <i className="fa-solid fa-table-list"></i>
          </button>
          <button onClick={toggleRightPanel} className={clsx('p-2 rounded-lg border text-xs transition', rightPanelVisible ? 'border-indigo-500/50 text-indigo-400 bg-indigo-500/10' : 'border-slate-800 text-slate-500 hover:text-slate-300')} title={t('header.toggleRight')}>
            <i className="fa-solid fa-window-maximize"></i>
          </button>
        </div>
      </div>
    </header>
  );
}
