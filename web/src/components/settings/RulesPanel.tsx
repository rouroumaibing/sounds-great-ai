import { useCallback, useEffect, useState } from 'react';
import clsx from 'clsx';
import { settingsService } from '../../services/settingsService';
import { useBreeds } from '../../hooks/useBreeds';
import { useI18n } from '../../store/useI18n';
import type { RulesData, HookManifestData } from '../../types';

type Tab = 'lifecycle' | 'agent-rules';

export function RulesPanel() {
  const { t } = useI18n();
  const [tab, setTab] = useState<Tab>('lifecycle');

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">{t('rules.title')}</h2>
        <p className="text-xs text-slate-400 mt-1">{t('rules.desc')}</p>
      </div>

      {/* Sub-tabs */}
      <div className="flex items-center space-x-1 border-b border-slate-800/60">
        <TabButton active={tab === 'lifecycle'} onClick={() => setTab('lifecycle')} icon="fa-solid fa-code-branch" label={t('rules.lifecycle')} />
        <TabButton active={tab === 'agent-rules'} onClick={() => setTab('agent-rules')} icon="fa-solid fa-book" label={t('rules.agentRules')} />
      </div>

      {tab === 'lifecycle' && <LifecycleTab />}
      {tab === 'agent-rules' && <AgentRulesTab />}
    </div>
  );
}

function TabButton({ active, onClick, icon, label }: { active: boolean; onClick: () => void; icon: string; label: string }) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'px-4 py-2 text-xs font-semibold transition border-b-2 -mb-px',
        active ? 'text-indigo-300 border-indigo-400' : 'text-slate-400 border-transparent hover:text-slate-200',
      )}
    >
      <i className={clsx(icon, 'mr-1.5')}></i>
      {label}
    </button>
  );
}

function LifecycleTab() {
  const { t } = useI18n();
  const { dogs } = useBreeds();
  const [manifest, setManifest] = useState<HookManifestData | null>(null);
  const [loading, setLoading] = useState(true);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [preview, setPreview] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);

  const fetchManifest = useCallback(async () => {
    setLoading(true);
    try {
      const data = await settingsService.getHookManifest();
      setManifest(data);
    } catch { /* ignore */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchManifest(); }, [fetchManifest]);

  const handlePreview = async () => {
    setPreviewOpen(true);
    setPreviewLoading(true);
    try {
      const compiled = await settingsService.getCompilePreview(dogs[0]?.id ?? 'bianmu');
      setPreview(compiled);
    } catch {
      setPreview(`// ${t('common.error')}`);
    } finally {
      setPreviewLoading(false);
    }
  };

  if (loading) return <div className="text-center text-slate-500 text-xs py-8">{t('common.loading')}</div>;

  return (
    <div className="space-y-4">
      {/* Lifecycle flow */}
      {manifest?.stages && manifest.stages.length > 0 && (
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
          <div className="flex items-center space-x-2 mb-3">
            <i className="fa-solid fa-diagram-project text-indigo-400 text-xs"></i>
            <h4 className="text-xs font-bold text-slate-200">{t('rules.lifecycleFlow')}</h4>
          </div>
          <div className="flex items-center flex-wrap gap-2">
            {manifest.stages.map((stage, i) => (
              <div key={stage} className="flex items-center gap-2">
                <span className="px-3 py-1.5 rounded-xl bg-slate-800 border border-slate-700 text-[11px] font-mono text-slate-200">{stage}</span>
                {i < manifest.stages.length - 1 && <i className="fa-solid fa-arrow-right text-slate-600 text-[10px]"></i>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Hook list */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center justify-between">
          <div className="flex items-center space-x-2">
            <i className="fa-solid fa-hook text-amber-400 text-xs"></i>
            <h4 className="text-xs font-bold text-slate-200">{t('rules.hookList')}</h4>
          </div>
          <button onClick={handlePreview} className="px-3 py-1 rounded-lg bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 text-[11px] font-semibold hover:bg-indigo-500/30 transition">
            <i className="fa-solid fa-eye mr-1"></i>{t('rules.compilePreview')}
          </button>
        </div>
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-slate-800/80 text-slate-400">
              <th className="text-left px-4 py-2 font-semibold">ID</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.name')}</th>
              <th className="text-left px-4 py-2 font-semibold">Stage</th>
              <th className="text-left px-4 py-2 font-semibold">Order</th>
              <th className="text-left px-4 py-2 font-semibold">Resolver</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.status')}</th>
            </tr>
          </thead>
          <tbody>
            {manifest?.hooks.map((hk) => (
              <tr key={hk.id} className="border-b border-slate-800/40">
                <td className="px-4 py-2.5 font-mono text-indigo-300 text-[11px]">{hk.id}</td>
                <td className="px-4 py-2.5 text-slate-200">{hk.name}</td>
                <td className="px-4 py-2.5 font-mono text-slate-400 text-[11px]">{hk.stage}</td>
                <td className="px-4 py-2.5 text-slate-400">{hk.order}</td>
                <td className="px-4 py-2.5 font-mono text-slate-400 text-[11px]">{hk.resolver}</td>
                <td className="px-4 py-2.5">
                  {hk.enabled ? (
                    <span className="px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 text-[10px] font-semibold">{t('rules.enabled')}</span>
                  ) : (
                    <span className="px-2 py-0.5 rounded bg-slate-700 text-slate-400 text-[10px] font-semibold">{t('rules.disabled')}</span>
                  )}
                  {!hk.disableable && <span className="ml-1 text-[9px] text-amber-400">{t('rules.notDisableable')}</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Compile preview modal */}
      {previewOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setPreviewOpen(false)}>
          <div className="bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl p-6 w-[700px] max-w-[90vw] max-h-[80vh] flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-bold text-slate-100">{t('rules.compilePreview')}</h3>
              <button onClick={() => setPreviewOpen(false)} className="text-slate-400 hover:text-slate-200">
                <i className="fa-solid fa-xmark"></i>
              </button>
            </div>
            <pre className="flex-1 overflow-auto text-[11px] font-mono text-slate-300 bg-slate-950/50 rounded-xl p-4 border border-slate-800">
              {previewLoading ? t('common.loading') : preview || t('rules.previewEmpty')}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

function AgentRulesTab() {
  const { t } = useI18n();
  const [rules, setRules] = useState<RulesData | null>(null);
  const [loading, setLoading] = useState(true);
  const [previewModal, setPreviewModal] = useState<{ title: string; content: string } | null>(null);

  useEffect(() => {
    settingsService.getRules()
      .then(setRules)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="text-center text-slate-500 text-xs py-8">{t('common.loading')}</div>;

  return (
    <div className="space-y-4">
      {/* Iron Laws */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
          <i className="fa-solid fa-shield-halved text-rose-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('rules.ironLaws')}</h4>
        </div>
        <div className="divide-y divide-slate-800/40">
          {rules?.iron_laws.map((law) => (
            <div key={law.id} className="px-4 py-3 flex items-start space-x-3">
              <span className="w-6 h-6 rounded-lg bg-rose-500/20 text-rose-300 border border-rose-500/30 text-[10px] font-bold flex items-center justify-center shrink-0">{law.id}</span>
              <div>
                <div className="text-xs font-bold text-slate-200">{law.title}</div>
                <div className="text-[11px] text-slate-400 mt-0.5 leading-relaxed">{law.desc}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Breed Restrictions */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
          <i className="fa-solid fa-dog text-amber-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('rules.dogLimits')}</h4>
        </div>
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-slate-800/80 text-slate-400">
              <th className="text-left px-4 py-2 font-semibold">{t('rules.dog')}</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.canDo')}</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.cannotDo')}</th>
            </tr>
          </thead>
          <tbody>
            {rules?.breed_restrictions.map((r) => (
              <tr key={r.breed} className="border-b border-slate-800/40">
                <td className="px-4 py-2.5 font-bold text-slate-200 whitespace-nowrap">{r.breed}</td>
                <td className="px-4 py-2.5 text-emerald-300 text-[11px]">{r.can}</td>
                <td className="px-4 py-2.5 text-rose-300 text-[11px]">{r.cannot}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Red Flags */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
          <i className="fa-solid fa-triangle-exclamation text-amber-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('rules.redFlags')}</h4>
        </div>
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-slate-800/80 text-slate-400">
              <th className="text-left px-4 py-2 font-semibold">{t('rules.redFlag')}</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.violation')}</th>
              <th className="text-left px-4 py-2 font-semibold">{t('rules.correctAction')}</th>
            </tr>
          </thead>
          <tbody>
            {rules?.red_flags.map((f, i) => (
              <tr key={i} className="border-b border-slate-800/40">
                <td className="px-4 py-2.5 text-rose-300 text-[11px] font-mono">{f.pattern}</td>
                <td className="px-4 py-2.5 text-slate-400 text-[11px] font-mono">{f.violation}</td>
                <td className="px-4 py-2.5 text-emerald-300 text-[11px]">{f.fix}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Model Guides */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
          <i className="fa-solid fa-book-open text-cyan-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">{t('rules.modelGuides')}</h4>
        </div>
        <div className="divide-y divide-slate-800/40">
          {rules?.model_guides.map((g) => (
            <div key={g.adapter} className="px-4 py-3 flex items-start space-x-3">
              <span className="px-2 py-0.5 rounded bg-cyan-500/20 text-cyan-300 text-[10px] font-mono font-bold shrink-0">{g.adapter}</span>
              <div className="text-[11px] text-slate-400 leading-relaxed">{g.guide}</div>
            </div>
          ))}
        </div>
      </div>

      {/* AGENTS.md preview */}
      {rules?.agents_content && (
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-800/80 flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <i className="fa-solid fa-file-lines text-slate-400 text-xs"></i>
              <h4 className="text-xs font-bold text-slate-200">AGENTS.md</h4>
            </div>
            <button onClick={() => setPreviewModal({ title: 'AGENTS.md', content: rules.agents_content })} className="px-3 py-1 rounded-lg bg-slate-800 text-slate-300 text-[11px] font-semibold hover:bg-slate-700 transition">
              <i className="fa-solid fa-expand mr-1"></i>{t('rules.viewFull')}
            </button>
          </div>
          <pre className="px-4 py-3 text-[10px] font-mono text-slate-400 max-h-40 overflow-auto whitespace-pre-wrap">{rules.agents_content.slice(0, 1000)}{rules.agents_content.length > 1000 ? '\n...' : ''}</pre>
        </div>
      )}

      {/* Preview modal */}
      {previewModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setPreviewModal(null)}>
          <div className="bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl p-6 w-[700px] max-w-[90vw] max-h-[80vh] flex flex-col" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-bold text-slate-100">{previewModal.title}</h3>
              <button onClick={() => setPreviewModal(null)} className="text-slate-400 hover:text-slate-200">
                <i className="fa-solid fa-xmark"></i>
              </button>
            </div>
            <pre className="flex-1 overflow-auto text-[11px] font-mono text-slate-300 bg-slate-950/50 rounded-xl p-4 border border-slate-800 whitespace-pre-wrap">{previewModal.content}</pre>
          </div>
        </div>
      )}
    </div>
  );
}
