import { useEffect } from 'react';
import { useChatStore } from '../../store/useChatStore';
import { useBreeds } from '../../hooks/useBreeds';
import { useCustodyTrail } from '../../hooks/useCustodyTrail';
import { resolveBreedDisplayName } from '../../lib/breed-display-name';
import type { TrailEntry } from '../../services/custody';

const EVENT_LABELS: Record<string, string> = {
  'ball.handed': '接球',
  'ball.held': '持球挂起',
  'wake_condition_met': '唤醒条件满足',
  'hold_dispositioned': '持球释放',
  'invocation.started': '开始执行',
  'invocation.heartbeat': '心跳',
  'invocation.died': '调用崩溃',
  'dispatch_dispositioned': '传球',
  'task.done': '完成',
  'task.blocked': '阻塞',
};

const STATE_LABELS: Record<string, string> = {
  new: '新建',
  active: '进行中',
  blocked: '阻塞',
  parked: '持球挂起',
  dead: '终止',
  void: '作废',
  zombie: '僵尸',
  resolved: '已解决',
};

const STATE_BADGE: Record<string, string> = {
  new: 'bg-slate-700 text-slate-200 border-slate-600',
  active: 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40',
  blocked: 'bg-amber-600/20 text-amber-300 border-amber-500/40',
  parked: 'bg-sky-600/20 text-sky-300 border-sky-500/40',
  dead: 'bg-rose-600/20 text-rose-300 border-rose-500/40',
  void: 'bg-slate-700 text-slate-400 border-slate-600',
  zombie: 'bg-rose-600/20 text-rose-300 border-rose-500/40',
  resolved: 'bg-emerald-600/20 text-emerald-300 border-emerald-500/40',
};

function describe(e: TrailEntry, getName: (id: string) => string): string {
  const who = (id?: string) => (id ? getName(id) : '—');
  switch (e.type) {
    case 'ball.handed':
      return `${who(e.from)} → ${who(e.to)}`;
    case 'dispatch_dispositioned':
      return `${who(e.from)} → ${who(e.to)}`;
    case 'ball.held':
      return who(e.holder);
    case 'hold_dispositioned':
      return who(e.holder);
    case 'invocation.started':
    case 'invocation.died':
      return who(e.holder);
    default:
      return who(e.holder) !== '—' ? who(e.holder) : '';
  }
}

export function CustodyTrail({ threadId }: { threadId: string }) {
  const { breeds } = useBreeds();
  const getName = (id: string) => resolveBreedDisplayName(id, (bid) => breeds.find((b) => b.id === bid));

  const { briefing, loading, error, refresh } = useCustodyTrail(threadId);

  // Re-fetch the trail when the orchestration emits new events for this thread
  // (BarkStart/BarkResult append to the chat store) so the audit trail stays live.
  const events = useChatStore((s) => s.events[threadId]);
  const isGenerating = useChatStore((s) => s.isGenerating[threadId] ?? false);
  const eventCount = events?.length ?? 0;
  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [threadId, eventCount]);

  // Derive the live "who holds the ball right now" from the streaming state.
  let liveHolder: string | undefined;
  if (isGenerating && events) {
    for (let i = events.length - 1; i >= 0; i--) {
      const ev = events[i];
      if (ev.type === 'breed_response_start') {
        liveHolder = (ev as { breed?: string }).breed;
        break;
      }
    }
  }

  const state = briefing?.state ?? 'new';
  const holder = liveHolder ?? briefing?.holder;

  return (
    <div className="flex-1 flex flex-col p-3 overflow-y-auto space-y-3">
      <div className="flex items-center justify-between border-b border-slate-800/80 pb-2">
        <span className="text-xs font-bold uppercase tracking-wider text-slate-200 flex items-center gap-1.5">
          <i className="fa-solid fa-circle-nodes text-indigo-400"></i>
          球权轨迹
        </span>
        <button
          onClick={() => refresh()}
          className="text-[10px] font-mono text-slate-400 hover:text-indigo-300 transition-colors"
          title="刷新"
        >
          <i className={`fa-solid fa-rotate ${loading ? 'animate-spin' : ''}`}></i>
        </button>
      </div>

      {error && (
        <div className="text-[11px] text-rose-300 bg-rose-600/10 border border-rose-500/30 rounded-lg p-2">
          轨迹加载失败：{error}
        </div>
      )}

      {!error && (
        <>
          {/* Live state / turn indicator */}
          <div className="p-2.5 rounded-xl border border-slate-700/60 bg-slate-950/60 space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="text-[10px] uppercase tracking-wider text-slate-500">当前状态</span>
              <span className={`px-2 py-0.5 rounded text-[10px] font-mono border ${STATE_BADGE[state] ?? STATE_BADGE.new}`}>
                {STATE_LABELS[state] ?? state}
              </span>
            </div>
            <div className="flex items-center gap-2 text-[11px]">
              <i className="fa-solid fa-paw text-indigo-400"></i>
              <span className="text-slate-300">
                {isGenerating ? (
                  <>持球中：<span className="font-mono text-indigo-300">{holder ? getName(holder) : '—'}</span></>
                ) : holder ? (
                  <>最近持球：<span className="font-mono text-slate-200">{getName(holder)}</span></>
                ) : (
                  <span className="text-slate-500">暂无持球</span>
                )}
              </span>
            </div>
          </div>

          {/* Summary stats */}
          {briefing && (
            <div className="grid grid-cols-3 gap-2 text-center">
              <Stat label="回合" value={briefing.turns} />
              <Stat label="传球" value={briefing.handoffs} />
              <Stat label="持球" value={briefing.holds} />
            </div>
          )}

          {/* Ordered trail */}
          <div className="space-y-1.5">
            <span className="text-[10px] uppercase tracking-wider text-slate-500">事件轨迹</span>
            {briefing && briefing.trail.length === 0 && (
              <div className="text-[11px] text-slate-500 py-2">该线程暂无球权事件。</div>
            )}
            {briefing?.trail.map((e) => (
              <div
                key={e.seq}
                className="flex items-start gap-2 text-[11px] p-2 rounded-lg border border-slate-800/60 bg-slate-950/40"
              >
                <span className="mt-0.5 px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 font-mono text-[9px] shrink-0">
                  {EVENT_LABELS[e.type] ?? e.type}
                </span>
                <span className="text-slate-300 font-mono">{describe(e, getName)}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="p-2 rounded-xl border border-slate-800/60 bg-slate-950/60">
      <div className="text-base font-bold text-slate-100 font-mono">{value}</div>
      <div className="text-[9px] uppercase tracking-wider text-slate-500">{label}</div>
    </div>
  );
}
