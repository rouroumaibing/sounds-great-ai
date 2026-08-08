import clsx from 'clsx';
import { useEffect, useRef, useState } from 'react';
import { Markdown } from '../common/Markdown';

interface ThinkingBlockProps {
  thinking: string;
  showThinking: boolean;
  isRunning?: boolean;
  status?: 'running' | 'success' | 'error';
}

export function ThinkingBlock({ thinking, showThinking: initialShow, isRunning = false, status }: ThinkingBlockProps) {
  const [showThinking, setShowThinking] = useState(initialShow);
  const userOverride = useRef(false);

  useEffect(() => {
    if (userOverride.current) return;
    setShowThinking(isRunning);
  }, [isRunning]);

  const handleToggle = () => {
    userOverride.current = true;
    setShowThinking((prev) => !prev);
  };

  const statusIcon = status === 'error'
    ? 'fa-circle-exclamation text-rose-400'
    : status === 'success'
    ? 'fa-check text-emerald-400'
    : isRunning
    ? 'fa-spinner fa-spin text-amber-400'
    : 'fa-brain text-indigo-400';

  return (
    <div className="bg-slate-950/80 rounded-xl border border-slate-800/80 text-xs overflow-hidden my-2">
      <button
        onClick={handleToggle}
        className={clsx(
          'w-full px-3 py-1.5 flex items-center justify-between text-slate-400 hover:text-slate-200 font-mono text-[11px] bg-slate-900/40',
          isRunning && 'animate-pulse'
        )}
      >
        <span className="flex items-center gap-1.5">
          <i className={clsx('fa-solid', statusIcon)}></i>
          <span>Thinking{isRunning ? '...' : status === 'success' ? ' (done)' : status === 'error' ? ' (error)' : ''}</span>
        </span>
        <i className={clsx('fa-solid text-[10px] transition-transform', showThinking ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
      </button>
      {showThinking && (
        <div className="p-3 border-t border-slate-800/80 border-l-2 border-l-indigo-500/50 font-mono text-[11px] text-slate-400 bg-slate-950/90 leading-relaxed">
          <Markdown>{thinking}</Markdown>
        </div>
      )}
    </div>
  );
}
