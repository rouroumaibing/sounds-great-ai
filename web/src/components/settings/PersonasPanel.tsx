import clsx from 'clsx';
import { useMemo, useState } from 'react';
import { useBreeds } from '../../hooks/useBreeds';
import { breedService } from '../../services/breedService';
import { useAppStore } from '../../store/useAppStore';
import { getBreedColor } from '../../lib/breed-colors';
import { HubBreedEditor } from './HubBreedEditor';
import type { BreedConfig } from '../../types/api';
import { useI18n } from '../../store/useI18n';

// --- helpers ---

function providerLabel(clientId: string): string {
  const t = useI18n.getState().t;
  const map: Record<string, string> = {
    claude: 'Claude (Anthropic)',
    codex: 'Codex (OpenAI)',
    gemini: 'Gemini (Google)',
    opencode: 'opencode',
    local: t('personas.localModel'),
  };
  return map[clientId] ?? clientId;
}

function formatModel(model: string): string {
  const t = useI18n.getState().t;
  if (!model || model === 'unknown') return t('personas.unknownModel');
  if (model.startsWith('claude-')) {
    const parts = model.replace('claude-', '').split('-');
    const vStart = parts.findIndex((p) => /^\d/.test(p));
    if (vStart > 0) {
      const name = parts.slice(0, vStart).map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
      const ver = parts.slice(vStart).join('.');
      return `Claude ${name} ${ver}`;
    }
    return `Claude ${parts.map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ')}`;
  }
  if (model.startsWith('gpt-')) return `GPT ${model.replace('gpt-', '')}`;
  if (model.startsWith('gemini-')) return model.split('-').map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
  return model;
}

// --- sub-components ---

function SummaryBar({ total, enabled, providers }: { total: number; enabled: number; providers: number }) {
  const { t } = useI18n();
  const pct = total > 0 ? Math.round((enabled / total) * 100) : 0;
  return (
    <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold text-slate-100">{t('personas.coverage')}</span>
          <span className={clsx(
            'px-2 py-0.5 rounded-lg text-[10px] font-mono font-bold border',
            pct >= 80 ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40'
              : pct >= 50 ? 'bg-amber-500/20 text-amber-300 border-amber-500/40'
              : 'bg-slate-800 text-slate-400 border-slate-700',
          )}>{pct}%</span>
        </div>
        <span className="text-xs text-slate-400">{total} {t('personas.dogs')} · {providers} {t('personas.providers')}</span>
      </div>
      <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-slate-800">
        <div className="h-full rounded-full bg-amber-500 transition-all" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function ProviderGroup({
  clientId,
  breeds,
  expanded,
  onToggle,
  onEdit,
  onToggleEnabled,
}: {
  clientId: string;
  breeds: BreedConfig[];
  expanded: boolean;
  onToggle: () => void;
  onEdit: (b: BreedConfig) => void;
  onToggleEnabled: (id: string, enabled: boolean) => void;
}) {
  const { t } = useI18n();
  const enabledCount = breeds.filter((b) => b.enabled).length;
  return (
    <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-slate-800/40 transition"
      >
        <div className="flex items-center gap-2.5">
          <i className={clsx('fa-solid fa-chevron-down text-xs text-slate-500 transition-transform', !expanded && '-rotate-90')} />
          <span className="text-sm font-bold text-slate-100">{providerLabel(clientId)}</span>
          <span className="px-2 py-0.5 rounded-lg bg-slate-800 border border-slate-700 text-[10px] font-mono text-slate-300">
            {breeds.length} {t('personas.dogUnit')} · {enabledCount} {t('personas.enabledUnit')}
          </span>
        </div>
      </button>
      {expanded && (
        <div className="border-t border-slate-800/80 divide-y divide-slate-800/60">
          {breeds.map((p) => {
            const color = p.color?.primary ?? getBreedColor(p.id).primary;
            const icon = p.avatar ?? '';
            return (
              <div key={p.id} className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex items-center space-x-3 min-w-0">
                    <div className="w-10 h-10 rounded-xl flex items-center justify-center text-white shadow shrink-0" style={{ backgroundColor: color }}>
                      <i className={icon}></i>
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-bold text-slate-100">{p.name}</span>
                        {p.nickname && <span className="text-[11px] text-slate-400">{p.nickname}</span>}
                      </div>
                      <div className="text-[11px] text-slate-400 font-mono">{p.display_name}</div>
                    </div>
                  </div>
                  <div className="flex items-center space-x-2 shrink-0">
                    <button onClick={() => onEdit(p)} className="p-1.5 text-slate-500 hover:text-amber-400 transition" title={t('personas.editDog')}>
                      <i className="fa-solid fa-pen text-xs"></i>
                    </button>
                    <button
                      onClick={() => onToggleEnabled(p.id, !p.enabled)}
                      className={clsx('w-11 h-6 rounded-full p-0.5 transition-colors relative focus:outline-none', p.enabled ? 'bg-amber-600' : 'bg-slate-800')}
                    >
                      <div className={clsx('w-5 h-5 rounded-full bg-white shadow-md transform transition-transform', p.enabled ? 'translate-x-5' : 'translate-x-0')} />
                    </button>
                  </div>
                </div>
                {/* one-liner */}
                {p.role_description && (
                  <p className="mt-2 text-xs text-slate-300 leading-relaxed">{p.role_description}</p>
                )}
                {/* personality */}
                <p className="mt-1 text-[11px] text-slate-400 leading-relaxed">{p.personality}</p>
                {/* strengths */}
                {p.team_strengths && (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {p.team_strengths.split(/[,，、]/).filter(Boolean).map((s) => (
                      <span key={s} className="px-1.5 py-0.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-[10px] text-emerald-300">{s.trim()}</span>
                    ))}
                  </div>
                )}
                {/* variants */}
                <div className="mt-2 space-y-1">
                  {(p.variants ?? []).map((v) => (
                    <div key={v.id} className="flex items-center gap-2 text-[10px] font-mono text-slate-500">
                      <span className="px-1.5 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-300">{v.id}</span>
                      <span>{formatModel(v.default_model)}</span>
                      <span>· mcp: {String(v.mcp_support)}</span>
                      {v.cli?.command && <span>· cli: {v.cli.command}</span>}
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// --- main ---

export function PersonasPanel() {
  const { t } = useI18n();
  const { breeds, loading, error, toggleEnabled, refetch } = useBreeds();
  const [editingBreed, setEditingBreed] = useState<BreedConfig | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const showToast = useAppStore((s) => s.showToast);

  const handleSaveBreed = async (breed: BreedConfig) => {
    try {
      if (editingBreed) {
        await breedService.updateBreed(editingBreed.id, breed);
        showToast({ message: t('personas.updated'), type: 'success' });
      } else {
        await breedService.createBreed(breed);
        showToast({ message: t('personas.created'), type: 'success' });
      }
      await refetch();
      setEditingBreed(null);
      setShowCreate(false);
    } catch {
      showToast({ message: t('personas.saveFailed'), type: 'error' });
    }
  };

  // group breeds by provider (client_id from first variant)
  const groups = useMemo(() => {
    const map = new Map<string, BreedConfig[]>();
    for (const b of breeds) {
      const key = b.variants?.[0]?.client_id ?? 'unknown';
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(b);
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [breeds]);

  const enabledCount = breeds.filter((b) => b.enabled).length;
  const allExpanded = groups.every(([k]) => expandedGroups.has(k));

  const toggleGroup = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const toggleAll = () => {
    if (allExpanded) setExpandedGroups(new Set());
    else setExpandedGroups(new Set(groups.map(([k]) => k)));
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-4">
      {/* action bar */}
      <div className="flex items-center justify-end gap-2">
        {groups.length > 0 && (
          <button onClick={toggleAll} className="px-3 py-1.5 rounded-lg text-[11px] text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 transition">
            {allExpanded ? t('personas.collapseAll') : t('personas.expandAll')}
          </button>
        )}
        <button onClick={() => setShowCreate(true)} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-lg shadow-amber-600/20">
          <i className="fa-solid fa-plus"></i><span>{t('personas.create')}</span>
        </button>
      </div>

      {/* loading / error */}
      {loading && <div className="text-center text-slate-500 text-xs py-8">{t('common.loading')}</div>}
      {error && <div className="text-center text-rose-400 text-xs py-8">{t('common.error')}</div>}

      {/* summary */}
      {!loading && !error && breeds.length > 0 && (
        <SummaryBar total={breeds.length} enabled={enabledCount} providers={groups.length} />
      )}

      {/* empty state */}
      {!loading && !error && breeds.length === 0 && (
        <div className="text-center py-12">
          <i className="fa-solid fa-id-card text-3xl text-slate-700 mb-3"></i>
          <p className="text-sm text-slate-500">{t('personas.empty')}</p>
          <p className="text-xs text-slate-600 mt-1">{t('personas.emptyHint')}</p>
        </div>
      )}

      {/* provider groups */}
      {groups.map(([clientId, groupBreeds]) => (
        <ProviderGroup
          key={clientId}
          clientId={clientId}
          breeds={groupBreeds}
          expanded={expandedGroups.has(clientId)}
          onToggle={() => toggleGroup(clientId)}
          onEdit={setEditingBreed}
          onToggleEnabled={toggleEnabled}
        />
      ))}

      {/* modals */}
      {showCreate && <HubBreedEditor onSave={handleSaveBreed} onClose={() => setShowCreate(false)} />}
      {editingBreed && <HubBreedEditor breed={editingBreed} onSave={handleSaveBreed} onClose={() => setEditingBreed(null)} />}
    </div>
  );
}
