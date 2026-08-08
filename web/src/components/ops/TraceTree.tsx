import { useState, useMemo } from 'react';
import clsx from 'clsx';
import type { OpsSpan } from '../../hooks/useOpsTraces';

interface TraceTreeProps {
  spans: OpsSpan[];
  traceId: string;
  onSelectSpan?: (span: OpsSpan) => void;
}

interface SpanNode {
  span: OpsSpan;
  children: SpanNode[];
  depth: number;
}

function toMs(t: string): number {
  return new Date(t).getTime();
}

export function TraceTree({ spans, traceId, onSelectSpan }: TraceTreeProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  // 按 parentID 构建树
  const tree = useMemo(() => {
    const childrenMap = new Map<string, OpsSpan[]>();

    for (const s of spans) {
      const parent = s.ParentID || 'root';
      if (!childrenMap.has(parent)) childrenMap.set(parent, []);
      childrenMap.get(parent)!.push(s);
    }

    const build = (span: OpsSpan, depth: number): SpanNode => ({
      span,
      depth,
      children: (childrenMap.get(span.SpanID) || [])
        .sort((a, b) => toMs(a.StartTime) - toMs(b.StartTime))
        .map((c) => build(c, depth + 1)),
    });

    const roots = (childrenMap.get('root') || [])
      .sort((a, b) => toMs(a.StartTime) - toMs(b.StartTime))
      .map((s) => build(s, 0));
    return roots;
  }, [spans]);

  // 计算 trace 总时长用于 duration bar 比例
  const traceDuration = useMemo(() => {
    if (spans.length === 0) return 1;
    const minStart = Math.min(...spans.map((s) => toMs(s.StartTime)));
    const maxEnd = Math.max(...spans.map((s) => toMs(s.EndTime)));
    return Math.max(maxEnd - minStart, 1);
  }, [spans]);

  const traceStart = useMemo(() => {
    if (spans.length === 0) return 0;
    return Math.min(...spans.map((s) => toMs(s.StartTime)));
  }, [spans]);

  const toggleExpand = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const renderNode = (node: SpanNode): React.ReactNode => {
    const duration = toMs(node.span.EndTime) - toMs(node.span.StartTime);
    const offset = toMs(node.span.StartTime) - traceStart;
    const widthPct = (duration / traceDuration) * 100;
    const offsetPct = (offset / traceDuration) * 100;
    const hasChildren = node.children.length > 0;
    const isExpanded = expanded.has(node.span.SpanID);

    return (
      <div key={node.span.SpanID}>
        <div
          className="flex items-center gap-2 py-1 hover:bg-slate-800/40 cursor-pointer rounded"
          style={{ paddingLeft: `${node.depth * 16 + 8}px` }}
          onClick={() => onSelectSpan?.(node.span)}
        >
          {hasChildren ? (
            <button
              onClick={(e) => { e.stopPropagation(); toggleExpand(node.span.SpanID); }}
              className="text-slate-500 hover:text-slate-300 text-[10px] w-4"
            >
              {isExpanded ? '▼' : '▶'}
            </button>
          ) : (
            <span className="w-4" />
          )}
          <span
            className={clsx(
              'text-[10px] font-mono w-1.5 h-1.5 rounded-full flex-shrink-0',
              node.span.Status === 'error' ? 'bg-rose-500' : 'bg-emerald-500'
            )}
          />
          <span className="text-[11px] text-slate-300 truncate flex-shrink-0 min-w-[120px] max-w-[200px]">
            {node.span.Name}
          </span>
          {/* duration bar */}
          <div className="flex-1 relative h-3 bg-slate-950 rounded overflow-hidden">
            <div
              className={clsx(
                'absolute h-full rounded',
                node.span.Status === 'error' ? 'bg-rose-500/60' : 'bg-indigo-500/60'
              )}
              style={{ left: `${offsetPct}%`, width: `${widthPct}%` }}
            />
          </div>
          <span className="text-[10px] text-slate-500 font-mono flex-shrink-0 w-16 text-right">
            {(duration / 1000).toFixed(2)}s
          </span>
        </div>
        {hasChildren && isExpanded && node.children.map(renderNode)}
      </div>
    );
  };

  if (spans.length === 0) {
    return <div className="text-center text-slate-500 text-xs py-4">无 span 数据</div>;
  }

  return (
    <div className="space-y-0.5">
      <div className="text-[10px] text-slate-500 font-mono mb-2">Trace: {traceId}</div>
      {tree.map(renderNode)}
    </div>
  );
}
