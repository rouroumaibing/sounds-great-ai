import { useState } from 'react';
import type { ApprovalRequestEvent } from '../../types';
import { useChatStore } from '../../store/useChatStore';

interface ApprovalBlockProps {
  event: ApprovalRequestEvent;
}

export function ApprovalBlock({ event }: ApprovalBlockProps) {
  const sendHitlResponse = useChatStore((s) => s.sendHitlResponse);
  const [reason, setReason] = useState('');

  const handleApprove = () => {
    sendHitlResponse(event.request_id, true, reason);
  };

  const handleReject = () => {
    sendHitlResponse(event.request_id, false, reason || 'rejected by user');
  };

  return (
    <div className="my-2 p-3 rounded-xl border border-amber-500/40 bg-amber-500/5 space-y-2">
      <div className="flex items-center gap-2 text-xs">
        <i className="fa-solid fa-shield-halved text-amber-400"></i>
        <span className="font-bold text-amber-300">需要人工审批</span>
        <span className="text-slate-500 font-mono text-[10px]">· {event.action}</span>
      </div>
      <p className="text-[11px] text-slate-400">{event.impact}</p>
      <input
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        placeholder="审批理由 (可选)"
        className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2 py-1 text-[11px] text-slate-200 focus:outline-none focus:border-amber-500"
      />
      <div className="flex items-center gap-2">
        <button
          onClick={handleApprove}
          className="px-3 py-1 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-[11px] font-medium transition"
        >
          <i className="fa-solid fa-check text-[9px] mr-1"></i>批准
        </button>
        <button
          onClick={handleReject}
          className="px-3 py-1 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-[11px] font-medium transition"
        >
          <i className="fa-solid fa-xmark text-[9px] mr-1"></i>拒绝
        </button>
      </div>
    </div>
  );
}
