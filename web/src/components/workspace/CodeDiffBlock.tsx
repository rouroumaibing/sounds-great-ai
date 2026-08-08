import { useState } from 'react';
import clsx from 'clsx';
import type { CodeDiffEvent } from '../../types';

interface CodeDiffBlockProps {
  event: CodeDiffEvent;
}

export function CodeDiffBlock({ event }: CodeDiffBlockProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="my-2 rounded-xl border border-slate-800 bg-slate-950 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/60 flex items-center justify-between hover:bg-slate-900/80 transition text-xs"
      >
        <div className="flex items-center gap-2">
          <i className="fa-solid fa-code-branch text-cyan-400 text-[10px]"></i>
          <span className="font-mono text-slate-200 font-bold">{event.file}</span>
          <span className="px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 text-[9px] font-mono">{event.action}</span>
        </div>
        <i className={clsx('fa-solid text-[9px] text-slate-500 transition-transform', expanded ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
      </button>
      {expanded && (
        <pre className="p-2.5 text-[10px] overflow-x-auto font-mono bg-slate-950/80 leading-normal whitespace-pre-wrap break-all text-slate-400">{event.diff}</pre>
      )}
    </div>
  );
}
