import { useCallback, useEffect, useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { listCapsules, getCapsule, getProposal } from '../../services/profilesService';
import { RelationshipCapsuleCard } from './RelationshipCapsuleCard';
import type { CapsuleSummary, RelationshipCapsule } from '../../types/profiles';

// ProfilesContent is the SG entry point for the relationship-capsule / 养熟
// approval loop (Persistent Identity P1/P1-b). It renders a relationship panel
// with an inline approval card, re-skinned to SG's dark theme. It owns the
// list/detail state and reloads after any decision.
export function ProfilesContent() {
  const { t } = useI18n();
  const [list, setList] = useState<CapsuleSummary[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [capsule, setCapsule] = useState<RelationshipCapsule | null>(null);
  const [proposal, setProposal] = useState<RelationshipCapsule | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const loadList = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await listCapsules();
      setList(data);
      if (!selected && data.length) setSelected(data[0].relationship_key);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, [selected]);

  const loadDetail = useCallback(async (key: string) => {
    setLoading(true);
    try {
      const [c, p] = await Promise.all([getCapsule(key), getProposal(key)]);
      setCapsule(c);
      setProposal(p);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'load failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadList();
  }, [loadList]);

  useEffect(() => {
    if (selected) loadDetail(selected);
  }, [selected, loadDetail]);

  const handleChanged = useCallback(() => {
    if (selected) loadDetail(selected);
    loadList();
  }, [selected, loadDetail, loadList]);

  return (
    <div className="h-full flex flex-col min-h-0 overflow-hidden bg-slate-950">
      <div className="px-4 py-3 border-b border-slate-800/80 flex items-center justify-between shrink-0">
        <div>
          <h2 className="text-base font-bold text-slate-100">
            <i className="fa-solid fa-paw text-amber-400 mr-2"></i>
            {t('profiles.title', '养熟 · 关系胶囊')}
          </h2>
          <p className="text-[11px] text-slate-500 mt-0.5">
            {t('profiles.subtitle', 'Persistent Identity · 狗狗与大当家的长期关系画像')}
          </p>
        </div>
        <button
          onClick={loadList}
          className="px-2 py-1 rounded-lg border border-slate-700 hover:border-amber-500 text-slate-300 text-[11px]"
        >
          <i className="fa-solid fa-rotate-right mr-1"></i>
          {t('profiles.refresh', '刷新')}
        </button>
      </div>

      <div className="flex-1 min-h-0 flex">
        <aside className="w-40 shrink-0 border-r border-slate-800/80 overflow-auto p-2 space-y-1">
          {list.map((c) => (
            <button
              key={c.relationship_key}
              onClick={() => setSelected(c.relationship_key)}
              className={`w-full text-left px-2 py-1.5 rounded-lg text-[12px] flex items-center justify-between gap-1 transition ${
                selected === c.relationship_key
                  ? 'bg-amber-500/15 text-amber-200'
                  : 'text-slate-300 hover:bg-slate-900'
              }`}
            >
              <span className="truncate">{c.relationship_key}</span>
              {c.pending_proposal && <span className="w-1.5 h-1.5 rounded-full bg-amber-400 shrink-0"></span>}
            </button>
          ))}
          {!list.length && (
            <p className="text-[11px] text-slate-500 px-2 py-1">{t('profiles.noKeys', '暂无关系键')}</p>
          )}
        </aside>

        <main className="flex-1 min-w-0 overflow-auto p-4">
          {loading && <p className="text-[12px] text-slate-500">{t('profiles.loading', '加载中…')}</p>}
          {error && <p className="text-[12px] text-rose-400">{error}</p>}
          {capsule && (
            <RelationshipCapsuleCard capsule={capsule} proposal={proposal} onChanged={handleChanged} />
          )}
        </main>
      </div>
    </div>
  );
}
