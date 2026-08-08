import type { ErrorEvent } from '../../types';

interface ErrorBlockProps {
  event: ErrorEvent;
}

export function ErrorBlock({ event }: ErrorBlockProps) {
  return (
    <div className="my-2 p-3 rounded-xl border border-rose-500/40 bg-rose-500/5 flex items-start gap-2 text-xs">
      <i className="fa-solid fa-circle-exclamation text-rose-400 mt-0.5"></i>
      <div>
        {event.breed && <span className="font-bold text-rose-300">{event.breed}: </span>}
        <span className="text-slate-300">{event.error}</span>
      </div>
    </div>
  );
}
