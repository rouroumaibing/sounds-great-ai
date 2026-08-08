import { useEffect, useRef } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';
import { CvoMessage } from './CvoMessage';
import { BreedCard } from './BreedCard';
import { SopGate } from './SopGate';
import { CvoEscalation } from './CvoEscalation';
import { CommandBar } from './CommandBar';
import { ThinkingBlock } from './ThinkingBlock';
import { ToolLogBlock } from './ToolLogBlock';
import { BreedResponseStart } from './BreedResponseStart';
import { BreedResponseComplete } from './BreedResponseComplete';
import { CodeDiffBlock } from './CodeDiffBlock';
import { ApprovalBlock } from './ApprovalBlock';
import { ErrorBlock } from './ErrorBlock';
import { TerminalOutputBlock } from './TerminalOutputBlock';

const EMPTY_EVENTS: never[] = [];

export function StreamTimeline() {
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const events = useChatStore((s) => s.events[activeThreadId] ?? EMPTY_EVENTS);

  const streamRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (streamRef.current) {
      streamRef.current.scrollTop = streamRef.current.scrollHeight;
    }
  }, [events.length]);

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <div ref={streamRef} className="flex-1 overflow-y-auto p-4 md:p-6 space-y-3">
        {events.map((event, idx) => {
          switch (event.type) {
            case 'cvo_message':
              return <CvoMessage key={idx} event={event} />;
            case 'breed_card':
              return <BreedCard key={idx} event={event} />;
            case 'sop_gate':
              return <SopGate key={idx} event={event} />;
            case 'cvo_escalation':
              return <CvoEscalation key={idx} event={event} />;
            case 'thinking':
              return <ThinkingBlock key={idx} thinking={event.content} showThinking={event.status === 'running'} isRunning={event.status === 'running'} status={event.status} />;
            case 'tool_call':
              return <ToolLogBlock key={idx} event={event} />;
            case 'code_diff':
              return <CodeDiffBlock key={idx} event={event} />;
            case 'terminal_output':
              return <TerminalOutputBlock key={idx} event={event} />;
            case 'approval_request':
              return <ApprovalBlock key={idx} event={event} />;
            case 'breed_response_start':
              return <BreedResponseStart key={idx} event={event} />;
            case 'breed_response_complete':
              return <BreedResponseComplete key={idx} event={event} />;
            case 'error':
              return <ErrorBlock key={idx} event={event} />;
            default:
              return null;
          }
        })}
      </div>
      <CommandBar />
    </div>
  );
}
