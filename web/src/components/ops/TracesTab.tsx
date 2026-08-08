import { useState } from "react";
import { useOpsTraces, type OpsSpan } from "../../hooks/useOpsTraces";
import { TraceTree } from "./TraceTree";
import { SpanDetail } from "./SpanDetail";
import { PromptXRay } from "./PromptXRay";

export function TracesTab() {
  const [traceId, setTraceId] = useState("");
  const [breedId, setBreedId] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [selectedSpan, setSelectedSpan] = useState<OpsSpan | null>(null);
  const [xraySpan, setXraySpan] = useState<OpsSpan | null>(null);
  const { traces, loading } = useOpsTraces(traceId, breedId);
  const selectedSpans = selected ? traces.spans.filter((s) => s.TraceID === selected) : [];

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <input
          value={traceId}
          onChange={(e) => setTraceId(e.target.value)}
          placeholder="traceId"
          className="bg-slate-950/60 border border-slate-800/80 rounded-lg px-2 py-1 text-xs text-slate-200 font-mono w-32"
        />
        <input
          value={breedId}
          onChange={(e) => setBreedId(e.target.value)}
          placeholder="breedId"
          className="bg-slate-950/60 border border-slate-800/80 rounded-lg px-2 py-1 text-xs text-slate-200 font-mono w-32"
        />
      </div>
      {traces.stats && (
        <div className="text-[10px] text-slate-500 font-mono">
          {traces.stats.Count} / {traces.stats.MaxSize} spans
        </div>
      )}
      {loading ? (
        <div className="text-slate-500 text-xs py-4 text-center">加载中...</div>
      ) : traces.spans.length === 0 ? (
        <div className="text-slate-500 text-xs py-4 text-center">暂无 trace 数据</div>
      ) : (
        <ul className="divide-y divide-slate-800/40">
          {traces.spans.map((s) => (
            <li
              key={s.SpanID}
              className="py-1.5 cursor-pointer hover:bg-slate-800/40 rounded px-1"
              onClick={() => {
                setSelected(s.TraceID === selected ? null : s.TraceID);
                setSelectedSpan(null);
              }}
            >
              <span className="font-mono text-[10px] text-slate-500">{s.TraceID.slice(0, 8)}</span>{" "}
              <span className="text-xs text-slate-200">{s.Name}</span>
              <span
                className={`ml-2 text-[10px] font-bold ${
                  s.Status === "error" ? "text-rose-400" : "text-emerald-400"
                }`}
              >
                {s.Status}
              </span>
            </li>
          ))}
        </ul>
      )}
      {selected && (
        <div className="mt-3 space-y-3">
          <div className="p-3 rounded-xl bg-slate-950/40 border border-slate-800">
            <TraceTree
              spans={selectedSpans}
              traceId={selected}
              onSelectSpan={(span) => setSelectedSpan(span)}
            />
          </div>
          <SpanDetail span={selectedSpan} onShowXRay={(span) => setXraySpan(span)} />
        </div>
      )}
      <PromptXRay span={xraySpan} onClose={() => setXraySpan(null)} />
    </div>
  );
}
