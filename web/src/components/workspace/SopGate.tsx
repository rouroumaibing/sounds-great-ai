import type { SopGateEvent } from '../../types';

interface SopGateProps {
  event: SopGateEvent;
}

export function SopGate({ event }: SopGateProps) {
  return (
    <div className="my-2 bg-amber-950/30 border border-amber-500/30 rounded-xl p-3 flex items-center justify-between text-xs text-amber-200">
      <div className="flex items-center space-x-2.5">
        <i className="fa-solid fa-shield-halved text-amber-400 text-sm"></i>
        <div>
          <span className="font-bold">[SOP Gate Triggered]:</span>
          <span className="text-amber-300/90 ml-1">{event.reason ?? 'Cross-Model Code Review 触发 - 正在自动路由至 灵缇 (Codex CLI) 进行严苛审查...'}</span>
        </div>
      </div>
      <span className="font-mono text-[10px] bg-amber-500/20 px-2 py-0.5 rounded border border-amber-500/30 text-amber-300">Rule: SOP-CROSS-REVIEW-02</span>
    </div>
  );
}
