import { useState } from 'react';
import clsx from 'clsx';
import { useAppStore } from '../../../store/useAppStore';
import { useSessionRecords } from '../../../hooks/useSessionRecords';
import { useBreeds } from '../../../hooks/useBreeds';
import type { SessionRecord } from '../../../types';
import { useI18n } from '../../../store/useI18n';

function ContextHealthBar({ ratio }: { ratio: number }) {
  const color = ratio < 0.5 ? 'bg-emerald-500' : ratio < 0.8 ? 'bg-amber-500' : 'bg-rose-500';
  return (
    <div className="w-full h-1.5 rounded-full bg-slate-800 overflow-hidden">
      <div className={clsx('h-full rounded-full transition-all', color)} style={{ width: `${Math.min(ratio * 100, 100)}%` }} />
    </div>
  );
}

function ActiveSessionCard({ record }: { record: SessionRecord }) {
  const { dogs } = useBreeds();
  const dog = dogs.find((d) => d.id === record.dogId);

  return (
    <div className="p-3 rounded-xl border border-emerald-500/30 bg-emerald-500/5 space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-mono font-bold text-emerald-400 bg-emerald-500/20 px-1.5 py-0.5 rounded">Active</span>
        <span className="text-[10px] font-mono text-slate-500">Session #{record.seq + 1}</span>
      </div>
      <div className="flex items-center gap-2 text-[11px]">
        <span className="font-mono text-slate-400 truncate">{(record.cliSessionId ?? '').slice(0, 16)}...</span>
        {dog && (
          <span className="px-1.5 py-0.5 rounded text-[9px] font-mono" style={{ backgroundColor: `${dog.color}20`, color: dog.color }}>
            {dog.name}
          </span>
        )}
      </div>
      <div className="text-[10px] text-slate-500 font-mono">
        {record.message_count} msgs · {record.compressionCount ?? 0} compress
      </div>
      <div className="flex items-center gap-3 text-[10px] font-mono text-slate-400">
        <span className="text-cyan-400">↓{(record.inputTokens ?? 0).toLocaleString()}</span>
        <span className="text-amber-400">↑{(record.outputTokens ?? 0).toLocaleString()}</span>
      </div>
      <ContextHealthBar ratio={record.contextFillRatio ?? 0} />
    </div>
  );
}

function SealedSessionRow({ record, onUnseal }: { record: SessionRecord; onUnseal: (id: string) => void }) {
  const { t } = useI18n();
  const { dogs } = useBreeds();
  const dog = dogs.find((d) => d.id === record.dogId);
  const timeAgo = record.sealed_at ? `${Math.round((Date.now() - record.sealed_at) / 60000)}m ago` : '';

  return (
    <div className="p-2.5 rounded-lg border border-slate-800 bg-slate-950/60 flex items-center justify-between">
      <div className="flex items-center gap-2 text-[11px]">
        <i className="fa-solid fa-lock text-slate-500 text-[10px]"></i>
        <span className="font-mono text-slate-400">Session #{record.seq + 1}</span>
        {dog && (
          <span className="px-1.5 py-0.5 rounded text-[9px] font-mono" style={{ backgroundColor: `${dog.color}20`, color: dog.color }}>
            {dog.name}
          </span>
        )}
        <span className="text-[10px] text-slate-500">{timeAgo} · {record.message_count} msgs</span>
      </div>
      <button onClick={() => onUnseal(record.id)} className="text-[10px] text-indigo-400 hover:text-indigo-300 transition font-mono">{t('drawer.sessionChain.unseal')}</button>
    </div>
  );
}

export function SessionChainTab() {
  const { t } = useI18n();
  const activeThreadId = useAppStore((s) => s.activeThreadId);
  const { sessions, loading, error, unseal } = useSessionRecords(activeThreadId);
  const [showSealed, setShowSealed] = useState(false);

  const activeSessions = sessions.filter((r) => r.status === 'active');
  const sealedSessions = sessions.filter((r) => r.status === 'sealed');

  if (loading) {
    return (
      <div className="text-center text-slate-500 text-xs py-8">
        <i className="fa-solid fa-spinner fa-spin text-lg text-slate-600 block mb-2"></i>
        {t('common.loading')}
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center text-rose-400 text-xs py-8">
        <i className="fa-solid fa-triangle-exclamation text-lg text-rose-500/50 block mb-2"></i>
        {t('common.error')}: {error}
      </div>
    );
  }

  if (sessions.length === 0) {
    return (
      <div className="text-center text-slate-500 text-xs py-8">
        <i className="fa-solid fa-link text-2xl text-slate-700 block mb-2"></i>
        {t('drawer.sessionChain.empty')}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-xs font-bold text-slate-200 flex items-center gap-1.5">
          <i className="fa-solid fa-link text-indigo-400"></i>
          {t('breedEditor.sessionChain')}
        </span>
        <span className="text-[10px] font-mono text-slate-500">
          {activeSessions.length} active · {sessions.length} total
        </span>
      </div>

      {activeSessions.map((record) => (
        <ActiveSessionCard key={record.id} record={record} />
      ))}

      {sealedSessions.length > 0 && (
        <div className="space-y-2">
          <button
            onClick={() => setShowSealed(!showSealed)}
            className="w-full px-3 py-1.5 rounded-lg border border-slate-800 bg-slate-950 text-[11px] font-mono text-slate-400 hover:text-slate-200 transition flex items-center justify-between"
          >
            <span className="flex items-center gap-1.5">
              <i className="fa-solid fa-lock text-[10px]"></i>
              {sealedSessions.length} sealed sessions
            </span>
            <i className={clsx('fa-solid text-[9px] transition-transform', showSealed ? 'fa-chevron-up' : 'fa-chevron-down')}></i>
          </button>
          {showSealed && (
            <div className="space-y-1.5">
              {sealedSessions.map((record) => (
                <SealedSessionRow key={record.id} record={record} onUnseal={unseal} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
