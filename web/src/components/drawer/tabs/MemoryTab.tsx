import { useMemory } from '../../../hooks/useMemory';
import { useI18n } from '../../../store/useI18n';

export function MemoryTab() {
  const { t } = useI18n();
  const { memories, loading, error } = useMemory();

  return (
    <div className="space-y-3">
      <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block">Shared Memory</span>
      {loading && <div className="text-center text-slate-500 text-xs py-4">{t('common.loading')}</div>}
      {error && <div className="text-center text-rose-400 text-xs py-4">{t('common.error')}: {error}</div>}
      {!loading && !error && memories.length === 0 && (
        <div className="text-center text-slate-500 text-xs py-4">{t('drawer.memory.empty')}</div>
      )}
      <div className="space-y-2">
        {memories.map((mem) => (
          <div key={mem.id} className="p-2.5 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1">
            <div className="flex items-center justify-between font-mono text-[10px]">
              <span className="text-amber-400 font-bold">[{mem.type}]</span>
              <span className="text-slate-500">{mem.time}</span>
            </div>
            <p className="text-[11px] text-slate-300">{mem.fact}</p>
            <div className="text-[9px] text-slate-500 font-mono text-right">By: {mem.author}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
