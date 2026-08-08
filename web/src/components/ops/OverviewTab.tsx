import { useOpsMetrics } from "../../hooks/useOpsMetrics";

function parseCounter(text: string, name: string): number {
  let total = 0;
  for (const line of text.split("\n")) {
    if (line.startsWith(name + "{") || line.startsWith(name + " ")) {
      const m = line.match(/\s(\d+(\.\d+)?)\s*$/);
      if (m) total += parseFloat(m[1]);
    }
  }
  return total;
}

function MetricCard({ label, value, suffix }: { label: string; value: number | string; suffix?: string }) {
  return (
    <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800/80">
      <div className="text-[10px] text-slate-400 font-semibold mb-1">{label}</div>
      <div className="text-sm font-mono font-bold text-slate-200">
        {value}
        {suffix ? ` ${suffix}` : ""}
      </div>
    </div>
  );
}

function TrendChart({ snapshots }: { snapshots: { timestamp: string; text: string }[] }) {
  const points = snapshots.slice(-60).map((s) => parseCounter(s.text, "dog_pack_invocation_completed"));
  if (points.length < 2) return <div className="text-slate-500 text-xs py-4 text-center">暂无趋势数据</div>;
  const max = Math.max(...points, 1);
  const w = 600;
  const h = 120;
  const path = points
    .map((v, i) => `${i === 0 ? "M" : "L"}${(i / (points.length - 1)) * w},${h - (v / max) * h}`)
    .join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-32">
      <polyline points={path.replace(/M|L/g, "").split(" ").reduce<string[]>((acc, p) => {
        const [x, y] = p.split(",");
        acc.push(`${x},${y}`);
        return acc;
      }, []).join(" ")} fill="none" stroke="#818cf8" strokeWidth="2" />
    </svg>
  );
}

export function OverviewTab() {
  const { snapshots, loading } = useOpsMetrics();
  const latest = snapshots[snapshots.length - 1]?.text ?? "";
  const ok = parseCounter(latest, "dog_pack_invocation_completed");
  const tokenTotal = parseCounter(latest, "dog_pack_token_usage");
  const handoffTotal = parseCounter(latest, "dog_pack_a2a_handoff_count");

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <MetricCard label="Invocation (ok)" value={ok} />
        <MetricCard label="Token usage" value={tokenTotal} />
        <MetricCard label="A2A Handoff" value={handoffTotal} />
        <MetricCard label="Snapshots" value={snapshots.length} />
      </div>
      <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80">
        <div className="text-[10px] text-slate-400 font-semibold mb-2">Invocation 趋势</div>
        {loading ? <div className="text-slate-500 text-xs py-4 text-center">加载中...</div> : <TrendChart snapshots={snapshots} />}
      </div>
    </div>
  );
}
