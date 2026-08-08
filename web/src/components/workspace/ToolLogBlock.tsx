import clsx from 'clsx';
import { useState } from 'react';
import type { ToolCallEvent } from '../../types';

interface ToolLogBlockProps {
  event: ToolCallEvent;
}

export function ToolLogBlock({ event }: ToolLogBlockProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="my-2 bg-slate-950 rounded-xl border border-slate-800 text-[11px] font-mono overflow-hidden shadow-inner">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/60 flex items-center justify-between hover:bg-slate-900/80 transition"
      >
        <div className="flex items-center space-x-2 text-cyan-400">
          <i className="fa-solid fa-gear text-[10px]"></i>
          <span className="font-bold">{event.tool}</span>
          <span className="text-slate-500">({event.params})</span>
        </div>
        <div className="flex items-center gap-2">
          <span className={clsx(
            'text-[10px] flex items-center gap-1',
            event.status === 'success' ? 'text-emerald-400' : event.status === 'error' ? 'text-rose-400' : 'text-amber-400'
          )}>
            <i className={clsx('fa-solid text-[9px]', event.status === 'success' ? 'fa-check' : event.status === 'error' ? 'fa-xmark' : 'fa-spinner fa-spin')}></i>
            {event.status}
          </span>
          <i className={clsx('fa-solid text-[9px] text-slate-500 transition-transform', expanded ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
        </div>
      </button>
      {expanded && (
        <pre className="p-2.5 text-slate-400 text-[10px] overflow-x-auto font-mono bg-slate-950/80 leading-normal whitespace-pre-wrap break-all">{event.result}</pre>
      )}
    </div>
  );
}
