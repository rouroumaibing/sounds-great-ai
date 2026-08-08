import clsx from 'clsx';
import { useEvals } from '../../hooks/useEvals';
import { useAppStore } from '../../store/useAppStore';

const verdictBadgeColors: Record<string, string> = {
  fix: 'bg-red-500/20 text-red-300 border-red-500/40',
  build: 'bg-blue-500/20 text-blue-300 border-blue-500/40',
  keep_observe: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
  delete_sunset: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
};

export function EvalPanel() {
  const { summaries, loading, error, triggerRun } = useEvals();
  const showToast = useAppStore((s) => s.showToast);

  if (loading) {
    return <div className="text-xs text-slate-400 p-4">加载评估数据...</div>;
  }
  if (error) {
    return <div className="text-xs text-red-400 p-4">加载失败: {error}</div>;
  }

  const total = summaries.length;
  const actionable = summaries.filter((s) => s.latestVerdict?.verdict === 'fix' || s.latestVerdict?.verdict === 'build').length;
  const keepObserve = summaries.filter((s) => s.latestVerdict?.verdict === 'keep_observe').length;
  const stale = summaries.filter((s) => !s.latestVerdict).length;

  return (
    <div className="max-w-5xl mx-auto w-full space-y-4">
      {/* 统计卡片 */}
      <div className="grid grid-cols-4 gap-3">
        {[
          { label: 'Total Domains', value: total, color: 'text-indigo-400' },
          { label: 'Actionable', value: actionable, color: 'text-red-400' },
          { label: 'Keep Observe', value: keepObserve, color: 'text-amber-400' },
          { label: 'Stale', value: stale, color: 'text-slate-400' },
        ].map((stat) => (
          <div key={stat.label} className="bg-slate-900/40 border border-slate-800 rounded-xl p-3 text-center">
            <div className={clsx('text-2xl font-bold', stat.color)}>{stat.value}</div>
            <div className="text-xs text-slate-500 mt-1">{stat.label}</div>
          </div>
        ))}
      </div>

      {/* Domain 卡片列表 */}
      <div className="space-y-3">
        {summaries.map((s) => (
          <div key={s.domain.domainId} className="bg-slate-900/40 border border-slate-800 rounded-xl p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <h3 className="text-sm font-bold text-slate-200">{s.domain.displayName}</h3>
                <p className="text-xs text-slate-500">{s.domain.descriptionForHuman}</p>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-slate-500">{s.domain.frequency}</span>
                <button
                  onClick={() => {
                    triggerRun(s.domain.domainId);
                    showToast({ message: `已触发 ${s.domain.displayName}`, type: 'success' });
                  }}
                  className="px-3 py-1 text-xs bg-indigo-600/20 text-indigo-300 border border-indigo-500/40 rounded-lg hover:bg-indigo-600/30"
                >
                  <i className="fa-solid fa-play text-xs mr-1"></i>触发
                </button>
              </div>
            </div>

            {/* Verdict 卡片 */}
            {s.latestVerdict ? (
              <div className="bg-slate-950/40 border border-slate-800/60 rounded-lg p-3 space-y-2">
                <div className="flex items-center gap-2">
                  <span className={clsx('text-xs px-2 py-0.5 rounded border font-mono', verdictBadgeColors[s.latestVerdict.verdict] || 'bg-slate-700 text-slate-300')}>
                    {s.latestVerdict.verdict}
                  </span>
                  <span className="text-xs text-slate-400">{s.latestVerdict.phenomenon}</span>
                </div>
                <div className="text-xs text-slate-500">
                  Verdict ID: {s.latestVerdict.id} · {new Date(s.latestVerdict.createdAt).toLocaleString()}
                </div>
              </div>
            ) : (
              <div className="text-xs text-slate-600 italic">暂无 verdict</div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
