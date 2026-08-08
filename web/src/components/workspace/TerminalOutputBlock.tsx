import { useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import type { TerminalOutputEvent } from '../../types';

interface TerminalOutputBlockProps {
  event: TerminalOutputEvent;
}

export function TerminalOutputBlock({ event }: TerminalOutputBlockProps) {
  const [expanded, setExpanded] = useState(true);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (preRef.current && expanded) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [event.data, expanded]);

  const isStderr = event.stream === 'stderr';

  return (
    <div className="my-2 rounded-xl border border-slate-800 bg-slate-950 overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/60 flex items-center justify-between hover:bg-slate-900/80 transition text-xs"
      >
        <div className="flex items-center gap-2">
          <i className={clsx('fa-solid text-[10px]', isStderr ? 'fa-circle-exclamation text-rose-400' : 'fa-terminal text-emerald-400')}></i>
          <span className="font-mono text-slate-200 font-bold">{event.stream}</span>
        </div>
        <i className={clsx('fa-solid text-[9px] text-slate-500 transition-transform', expanded ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
      </button>
      {expanded && (
        <pre
          ref={preRef}
          className={clsx(
            'p-2.5 text-[10px] overflow-x-auto font-mono bg-slate-950/80 leading-normal whitespace-pre-wrap break-all max-h-48 overflow-y-auto',
            isStderr ? 'text-rose-400' : 'text-emerald-400'
          )}
        >{event.data}</pre>
      )}
    </div>
  );
}
