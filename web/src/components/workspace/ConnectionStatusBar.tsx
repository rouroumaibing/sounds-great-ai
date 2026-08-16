import type { StreamEvent } from '../../types';
import { useAppStore } from '../../store/useAppStore';
import { useChatStore } from '../../store/useChatStore';

// Stable empty-array reference. The zustand selector below must never return a
// freshly-allocated `[]` on every snapshot check — that makes React's
// useSyncExternalStore believe the value changed each render and throws
// "Maximum update depth exceeded" (#185). Reuse this constant instead.
const EMPTY_EVENTS: StreamEvent[] = [];

// ConnectionStatusBar: three health pills — local API / socket
// / upstream model. API and socket track the live WS readyState; the upstream
// model pill is driven by the structured CARRIER_HEALTH WS event (T25 / R6)
// pushed from the backend's carrier-health store. Before the first health
// event arrives we fall back to the events-derived heuristic so we don't lose
// the pre-T25 behavior.
type HealthLevel = 'online' | 'degraded' | 'offline';

const LEVEL_STYLE: Record<HealthLevel, { dot: string; text: string; label: string }> = {
  online: { dot: 'bg-emerald-400', text: 'text-emerald-300', label: '畅通' },
  degraded: { dot: 'bg-amber-400', text: 'text-amber-300', label: '降级' },
  offline: { dot: 'bg-red-400', text: 'text-red-300', label: '离线' },
};

const ConnPill = ({ label, online }: { label: string; online: boolean }) => {
  const s = online
    ? LEVEL_STYLE.online
    : { dot: 'bg-slate-500', text: 'text-slate-400', label: '离线' };
  return (
    <div className="flex items-center gap-1.5">
      <span className={`w-2 h-2 rounded-full ${s.dot}`} />
      <span className="text-slate-400">{label}:</span>
      <span className={s.text}>{s.label}</span>
    </div>
  );
};

const LevelPill = ({
  label,
  level,
  title,
}: {
  label: string;
  level: HealthLevel;
  title?: string;
}) => {
  const s = LEVEL_STYLE[level];
  return (
    <div className="flex items-center gap-1.5" title={title}>
      <span className={`w-2 h-2 rounded-full ${s.dot}`} />
      <span className="text-slate-400">{label}:</span>
      <span className={s.text}>{s.label}</span>
    </div>
  );
};

export function ConnectionStatusBar() {
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const wsReadyState = useChatStore((s) => s.wsReadyState);
  const events = useChatStore((s) => s.events[activeThreadId] ?? EMPTY_EVENTS);
  const carrierHealth = useChatStore((s) => s.carrierHealth);

  const socketOnline = wsReadyState === 1; // WebSocket.OPEN

  // Determine upstream model health. Prefer the structured CARRIER_HEALTH map
  // (T25 / R6) when the backend has reported any carrier state; otherwise fall
  // back to the events-derived heuristic (a hard stall or error degrades).
  let upstreamLevel: HealthLevel = 'online';
  let upstreamReason: string | undefined;
  const carriers = Object.values(carrierHealth);
  if (carriers.length > 0) {
    let worst: HealthLevel = 'online';
    for (const c of carriers) {
      if (c.level === 'offline') worst = 'offline';
      else if (c.level === 'degraded' && worst === 'online') worst = 'degraded';
    }
    upstreamLevel = worst;
    const bad = carriers
      .filter((c) => c.level !== 'online' && c.reason)
      .sort((a, b) => b.updatedAt - a.updatedAt);
    upstreamReason = bad[0]?.reason;
  } else {
    for (let i = events.length - 1; i >= 0; i--) {
      const e = events[i] as { type: string; hard?: boolean; error?: string };
      if (e.type === 'breed_stall_warning' && e.hard) {
        upstreamLevel = 'degraded';
        break;
      }
      if (e.type === 'error') {
        upstreamLevel = 'degraded';
        break;
      }
      if (e.type === 'breed_response_complete' || e.type === 'breed_response_live') {
        // a healthy response clears prior degradation for display purposes
        break;
      }
    }
  }

  const hasIssue = !socketOnline || upstreamLevel !== 'online';

  return (
    <div
      className={`flex items-center gap-4 px-4 py-1.5 border-b text-[11px] ${
        hasIssue
          ? 'border-amber-800/50 bg-amber-950/20'
          : 'border-slate-800/60 bg-slate-900/40'
      }`}
    >
      <ConnPill label="本地 API" online={socketOnline} />
      <ConnPill label="Socket" online={socketOnline} />
      <LevelPill label="上游模型" level={upstreamLevel} title={upstreamReason} />
      {activeThreadId && (
        <span className="ml-auto text-slate-600 font-mono text-[10px] truncate max-w-[40%]">
          {activeThreadId}
        </span>
      )}
    </div>
  );
}
