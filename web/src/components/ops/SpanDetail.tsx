import clsx from 'clsx';
import type { OpsSpan } from '../../hooks/useOpsTraces';

interface SpanDetailProps {
  span: OpsSpan | null;
  onShowXRay?: (span: OpsSpan) => void;
}

function attrStr(span: OpsSpan, key: string): string {
  const v = span.Attributes[key];
  if (v === undefined || v === null) return '';
  return String(v);
}

export function SpanDetail({ span, onShowXRay }: SpanDetailProps) {
  if (!span) {
    return <div className="text-center text-slate-500 text-xs py-4">选择一个 span 查看详情</div>;
  }

  const duration = new Date(span.EndTime).getTime() - new Date(span.StartTime).getTime();
  const isCliInvoke = attrStr(span, 'cli.invoke') === 'true' || span.Attributes['prompt.system'] !== undefined;
  const errorMsg = attrStr(span, 'error.message');

  return (
    <div className="space-y-3 p-3 rounded-xl bg-slate-950/60 border border-slate-800">
      {/* Header */}
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <span className={clsx(
            'text-[10px] font-mono w-2 h-2 rounded-full',
            span.Status === 'error' ? 'bg-rose-500' : 'bg-emerald-500'
          )} />
          <span className="text-sm font-bold text-slate-200">{span.Name}</span>
        </div>
        <div className="text-[10px] text-slate-500 font-mono space-y-0.5">
          <div>Span ID: {span.SpanID}</div>
          <div>Parent: {span.ParentID || '(root)'}</div>
          <div>Duration: {(duration / 1000).toFixed(3)}s</div>
          <div>Start: {new Date(span.StartTime).toLocaleString('zh-CN')}</div>
        </div>
      </div>

      {/* Error */}
      {span.Status === 'error' && errorMsg && (
        <div className="p-2 rounded bg-rose-950/40 border border-rose-900 text-[11px] text-rose-300">
          <span className="font-bold">Error: </span>{errorMsg}
        </div>
      )}

      {/* Attributes table */}
      {Object.keys(span.Attributes).length > 0 && (
        <div className="space-y-1">
          <div className="text-[10px] uppercase tracking-wider text-slate-500 font-bold">Attributes</div>
          <div className="space-y-0.5">
            {Object.entries(span.Attributes).map(([k, v]) => (
              <div key={k} className="flex gap-2 text-[10px] font-mono">
                <span className="text-amber-400 flex-shrink-0 w-40 truncate">{k}</span>
                <span className="text-slate-400 truncate">{String(v)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* X-Ray button */}
      {isCliInvoke && onShowXRay && (
        <button
          onClick={() => onShowXRay(span)}
          className="w-full py-1.5 rounded bg-indigo-600/20 hover:bg-indigo-600/40 border border-indigo-700/50 text-indigo-300 text-[11px] font-medium transition"
        >
          🔍 Prompt X-Ray
        </button>
      )}
    </div>
  );
}
