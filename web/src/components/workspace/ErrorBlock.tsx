import { useState } from 'react';
import type { ErrorEvent } from '../../types';
import {
  classifyErrorTier,
  TIER_STYLES,
  safeExcerpt,
  redactMeta,
  sanitizePathLeaks,
} from '../../lib/diagnostics';

interface ErrorBlockProps {
  event: ErrorEvent;
}

// CliDiagnosticsPanel-style rendering: color-coded by
// reason tier (4-level palette), a meta bar with redacted context (paths, cli
// source), and a whitelist-gated collapsible "safe excerpt" of the (already
// server-sanitized) stderr. When structured diagnostics are absent the panel
// degrades to the prior behavior (show the error text + a collapsible copy).
export function ErrorBlock({ event }: ErrorBlockProps) {
  const [open, setOpen] = useState(false);
  const tier = classifyErrorTier(event.reason, event.summary || event.error);
  const style = TIER_STYLES[tier];

  // Prefer the public summary/hint (server-classified) for the headline; fall
  // back to the sanitized raw error text.
  const headline = event.summary
    ? sanitizePathLeaks(event.summary)
    : sanitizePathLeaks(event.error);
  const excerpt = safeExcerpt(event.excerpt, event.source);
  const meta = redactMeta(event.meta);

  const metaEntries = Object.entries(meta).filter(([, v]) => v && v !== '');

  return (
    <div className={`my-2 rounded-xl border ${style.border} ${style.bg}`}>
      <div className="p-3 flex items-start gap-2 text-xs">
        <i className={`fa-solid ${style.icon} ${style.text} mt-0.5`}></i>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            {event.breed && <span className="font-bold text-slate-200">{event.breed}</span>}
            <span
              className={`text-[10px] px-1.5 py-0.5 rounded-full border ${style.border} ${style.text} uppercase`}
            >
              {event.reason ? event.reason : tier}
            </span>
          </div>
          <span className="text-slate-300 break-words block mt-0.5">{headline}</span>
          {event.hint && (
            <span className="text-slate-400 break-words block mt-0.5 text-[11px]">
              提示: {event.hint}
            </span>
          )}
          {/* Meta bar: redacted context (paths / cli source). */}
          {(event.source || metaEntries.length > 0) && (
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-[10px] text-slate-500">
              {event.source && (
                <span className="px-1.5 py-0.5 rounded border border-slate-700/60 bg-slate-900/40 font-mono">
                  src: {event.source}
                </span>
              )}
              {metaEntries.map(([k, v]) => (
                <span
                  key={k}
                  className="px-1.5 py-0.5 rounded border border-slate-700/60 bg-slate-900/40 font-mono max-w-full truncate"
                  title={v}
                >
                  {k}: {v}
                </span>
              ))}
            </div>
          )}
        </div>
        <button
          onClick={() => setOpen((v) => !v)}
          className="text-slate-500 hover:text-slate-300 text-[10px] px-1.5 py-0.5 rounded border border-slate-700/60"
          title="查看详细错误"
        >
          {open ? '收起' : '详情'}
        </button>
      </div>
      {open && (
        <div className="px-3 pb-3">
          {excerpt.show ? (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-slate-800/80 bg-slate-950/60 px-3 py-2 text-[11px] leading-relaxed text-slate-400 font-mono">
{excerpt.text}
            </pre>
          ) : excerpt.text ? (
            <div className="rounded-lg border border-slate-800/80 bg-slate-950/60 px-3 py-2 text-[11px] leading-relaxed text-slate-500">
              {excerpt.text}
            </div>
          ) : (
            <pre className="whitespace-pre-wrap break-words rounded-lg border border-slate-800/80 bg-slate-950/60 px-3 py-2 text-[11px] leading-relaxed text-slate-400 font-mono">
{sanitizePathLeaks(event.error)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
