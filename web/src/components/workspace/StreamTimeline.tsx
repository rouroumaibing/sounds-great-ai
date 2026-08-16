import { useEffect, useRef } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { useChatHistory } from '../../hooks/useChatHistory';
import { getBreedColor } from '../../lib/breed-colors';
import { CvoMessage } from './CvoMessage';
import { BreedCard } from './BreedCard';
import { SopGate } from './SopGate';
import { CvoEscalation } from './CvoEscalation';
import { CommandBar } from './CommandBar';
import { ThinkingBlock } from './ThinkingBlock';
import { BreedResponseStart } from './BreedResponseStart';
import { BreedResponseComplete } from './BreedResponseComplete';
import { CodeDiffBlock } from './CodeDiffBlock';
import { ApprovalBlock } from './ApprovalBlock';
import { ErrorBlock } from './ErrorBlock';
import { CliOutputBlock } from './CliOutputBlock';
import type { ToolCallEvent, TerminalOutputEvent, StreamEvent } from '../../types';

const EMPTY_EVENTS: never[] = [];

// isCliRun returns true for the event types that belong inside a CLI run block
// (the cli_output / tool_event grouping).
function isCliRun(e: StreamEvent): boolean {
  return e.type === 'tool_call' || e.type === 'terminal_output';
}

// groupCliRuns collapses consecutive tool_call / terminal_output events into a
// single CliOutputBlock so a CLI invocation's tool calls and stdout render as
// one collapsible timeline card instead of a flat list.
function groupCliRuns(events: StreamEvent[]): Array<{ kind: 'single'; event: StreamEvent; idx: number } | { kind: 'cli'; tools: ToolCallEvent[]; terminals: TerminalOutputEvent[]; idx: number }> {
  const out: Array<{ kind: 'single'; event: StreamEvent; idx: number } | { kind: 'cli'; tools: ToolCallEvent[]; terminals: TerminalOutputEvent[]; idx: number }> = [];
  let i = 0;
  while (i < events.length) {
    if (isCliRun(events[i])) {
      const tools: ToolCallEvent[] = [];
      const terminals: TerminalOutputEvent[] = [];
      const startIdx = i;
      while (i < events.length && isCliRun(events[i])) {
        if (events[i].type === 'tool_call') tools.push(events[i] as ToolCallEvent);
        else terminals.push(events[i] as TerminalOutputEvent);
        i++;
      }
      out.push({ kind: 'cli', tools, terminals, idx: startIdx });
    } else {
      out.push({ kind: 'single', event: events[i], idx: i });
      i++;
    }
  }
  return out;
}

export function StreamTimeline() {
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const events = useChatStore((s) => s.events[activeThreadId] ?? EMPTY_EVENTS);
  const { hasMore, loadOlder } = useChatHistory(activeThreadId);

  const streamRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (streamRef.current) {
      streamRef.current.scrollTop = streamRef.current.scrollHeight;
    }
  }, [events.length]);

  // Scroll-up hydration: when the user scrolls near the top and more history
  // exists, load an earlier page (G9).
  const onScroll = () => {
    const el = streamRef.current;
    if (!el || !hasMore) return;
    if (el.scrollTop < 48) loadOlder();
  };

  const grouped = groupCliRuns(events);

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div ref={streamRef} onScroll={onScroll} className="flex-1 overflow-y-auto p-4 md:p-6 space-y-3">
        {grouped.map((g) => {
          if (g.kind === 'cli') {
            return <CliOutputBlock key={`cli-${g.idx}`} tools={g.tools} terminals={g.terminals} />;
          }
          const event = g.event;
          switch (event.type) {
            case 'cvo_message':
              return <CvoMessage key={g.idx} event={event} />;
            case 'breed_card':
              return <BreedCard key={g.idx} event={event} />;
            case 'sop_gate':
              return <SopGate key={g.idx} event={event} />;
            case 'cvo_escalation':
              return <CvoEscalation key={g.idx} event={event} />;
            case 'thinking':
              return <ThinkingBlock key={g.idx} thinking={event.content} showThinking={event.status === 'running'} isRunning={event.status === 'running'} status={event.status} />;
            case 'code_diff':
              return <CodeDiffBlock key={g.idx} event={event} />;
            case 'approval_request':
              return <ApprovalBlock key={g.idx} event={event} />;
            case 'breed_response_start':
              return <BreedResponseStart key={g.idx} event={event} />;
            case 'breed_response_complete':
              return <BreedResponseComplete key={g.idx} event={event} />;
            case 'breed_response_live': {
              const e = event as { breed: string; content: string };
              const color = getBreedColor(e.breed).primary;
              return (
                <div className="my-2" key={g.idx}>
                  <div className="flex items-center gap-2 text-xs">
                    <div
                      className="w-6 h-6 rounded-lg flex items-center justify-center text-white shadow"
                      style={{ backgroundColor: color }}
                    >
                      <i className="fa-solid fa-comment-dots text-[10px]"></i>
                    </div>
                    <span className="font-bold text-slate-200">{e.breed}</span>
                    <span className="text-sky-400 font-mono text-[10px]">responding…</span>
                    <span className="flex-1 h-px bg-slate-800"></span>
                  </div>
                  {e.content ? (
                    <div className="ml-8 mt-1 whitespace-pre-wrap rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 py-2 text-sm leading-relaxed text-slate-200">
                      {e.content}
                      <span className="inline-block w-2 h-4 ml-0.5 bg-sky-400/70 align-middle animate-pulse">
                        &nbsp;
                      </span>
                    </div>
                  ) : null}
                </div>
              );
            }
            case 'breed_stall_warning': {
              const e = event as { breed: string; state: string; hard: boolean; message: string };
              const color = getBreedColor(e.breed).primary;
              const isRecover = e.state === 'active';
              const tone = isRecover
                ? 'border-emerald-800/60 bg-emerald-900/20 text-emerald-300'
                : e.hard
                  ? 'border-red-800/60 bg-red-900/20 text-red-300'
                  : 'border-amber-800/60 bg-amber-900/20 text-amber-300';
              const icon = isRecover ? 'fa-check' : e.hard ? 'fa-triangle-exclamation' : 'fa-hourglass-half';
              return (
                <div className="my-1" key={g.idx}>
                  <div className={`flex items-center gap-2 text-xs rounded-lg px-3 py-1.5 border ${tone}`}>
                    <div
                      className="w-5 h-5 rounded flex items-center justify-center text-white text-[10px]"
                      style={{ backgroundColor: color }}
                    >
                      <i className={`fa-solid ${icon} text-[10px]`}></i>
                    </div>
                    <span className="font-bold">{e.breed}</span>
                    <span>{e.message}</span>
                  </div>
                </div>
              );
            }
            case 'error':
              return <ErrorBlock key={g.idx} event={event} />;
            default:
              return null;
          }
        })}
      </div>
      <CommandBar />
    </div>
  );
}
