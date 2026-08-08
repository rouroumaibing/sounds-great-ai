import type { CvoMessageEvent } from '../../types';
import { Markdown } from '../common/Markdown';

interface CvoMessageProps {
  event: CvoMessageEvent;
}

export function CvoMessage({ event }: CvoMessageProps) {
  return (
    <div className="flex justify-end my-3">
      <div className="ml-auto max-w-[85%] w-fit bg-indigo-600/20 border border-indigo-500/30 rounded-2xl rounded-br-sm p-3 text-slate-100 [&>p]:inline [&>p]:m-0 space-y-2">
        <div className="text-xs leading-relaxed">
          <Markdown>{event.content}</Markdown>
        </div>
        <div className="text-right">
          <span className="text-[10px] text-slate-500 font-mono">{event.timestamp}</span>
        </div>
      </div>
    </div>
  );
}
