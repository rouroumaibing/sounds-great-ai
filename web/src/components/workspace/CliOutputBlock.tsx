import { useState } from 'react';
import clsx from 'clsx';
import type { ToolCallEvent, TerminalOutputEvent } from '../../types';
import { ToolLogBlock } from './ToolLogBlock';

interface CliOutputBlockProps {
  tools: ToolCallEvent[];
  terminals: TerminalOutputEvent[];
  // Optional CLI label (e.g. the breed / cli command) for the header.
  label?: string;
}

// CliOutputBlock: collapses a CLI run's tool invocations and stdout/stderr into
// a single timeline card with a header summary, a collapsible tools section,
// and a stdout block. This groups consecutive tool_event / cli_output WS events
// instead of rendering each inline.
export function CliOutputBlock({ tools, terminals, label }: CliOutputBlockProps) {
  const [open, setOpen] = useState(true);

  const toolCount = tools.length;
  const hasStdout = terminals.some((t) => t.stream === 'stdout' && t.data.trim() !== '');
  const hasStderr = terminals.some((t) => t.stream === 'stderr' && t.data.trim() !== '');
  const errCount = tools.filter((t) => t.status === 'error').length;

  const headerTone = errCount > 0
    ? 'text-rose-300'
    : hasStderr
      ? 'text-amber-300'
      : 'text-slate-300';

  return (
    <div className="my-2 rounded-xl border border-slate-800 bg-slate-950/40 overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full px-3 py-1.5 bg-slate-900/60 border-b border-slate-800/60 flex items-center justify-between hover:bg-slate-900/80 transition text-xs"
      >
        <div className="flex items-center gap-2">
          <i className="fa-solid fa-terminal text-[10px] text-emerald-400"></i>
          <span className="font-bold text-slate-200">{label ?? 'CLI 运行'}</span>
          <span className={clsx('text-[10px]', headerTone)}>
            {toolCount > 0 && `${toolCount} 个工具`}
            {toolCount > 0 && (hasStdout || hasStderr) && ' · '}
            {hasStdout && 'stdout'}
            {hasStderr && ' · stderr'}
            {errCount > 0 && ` · ${errCount} 失败`}
          </span>
        </div>
        <i className={clsx('fa-solid text-[9px] text-slate-500 transition-transform', open ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
      </button>
      {open && (
        <div className="p-2 space-y-2">
          {tools.length > 0 && (
            <div className="space-y-2">
              {tools.map((t, i) => (
                <ToolLogBlock key={i} event={t} />
              ))}
            </div>
          )}
          {terminals.map((t, i) => (
            <div key={`t-${i}`} className="rounded-lg border border-slate-800 bg-slate-950 overflow-hidden">
              <div className="px-2.5 py-1 bg-slate-900/60 border-b border-slate-800/60 flex items-center gap-2 text-[10px]">
                <i
                  className={clsx(
                    'fa-solid',
                    t.stream === 'stderr' ? 'fa-circle-exclamation text-rose-400' : 'fa-terminal text-emerald-400'
                  )}
                ></i>
                <span className="font-mono text-slate-300 font-bold">{t.stream}</span>
              </div>
              <pre
                className={clsx(
                  'p-2 text-[10px] overflow-x-auto font-mono bg-slate-950/80 leading-normal whitespace-pre-wrap break-all max-h-48 overflow-y-auto',
                  t.stream === 'stderr' ? 'text-rose-400' : 'text-emerald-400'
                )}
              >{t.data}</pre>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
