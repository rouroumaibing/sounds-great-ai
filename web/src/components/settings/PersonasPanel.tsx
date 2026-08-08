import clsx from 'clsx';
import { useState } from 'react';
import { useBreeds } from '../../hooks/useBreeds';
import { breedService } from '../../services/breedService';
import { useAppStore } from '../../store/useAppStore';
import { getBreedColor } from '../../lib/breed-colors';
import { HubBreedEditor } from './HubBreedEditor';
import type { BreedConfig } from '../../types/api';

export function PersonasPanel() {
  const { breeds, loading, error, toggleEnabled, refetch } = useBreeds();
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [editingBreed, setEditingBreed] = useState<BreedConfig | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const showToast = useAppStore((s) => s.showToast);

  const handleSaveBreed = async (breed: BreedConfig) => {
    try {
      if (editingBreed) {
        await breedService.updateBreed(editingBreed.id, breed);
        showToast({ message: '犬种已更新', type: 'success' });
      } else {
        await breedService.createBreed(breed);
        showToast({ message: '犬种已创建', type: 'success' });
      }
      await refetch();
      setEditingBreed(null);
      setShowCreate(false);
    } catch {
      showToast({ message: '保存犬种失败', type: 'error' });
    }
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="flex items-start justify-between border-b border-slate-800/80 pb-5">
        <div>
          <h2 className="text-2xl font-bold text-slate-100">犬种画像</h2>
          <p className="text-xs text-slate-400 mt-1">犬种的个性、角色与能力配置。</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-lg shadow-amber-600/20">
          <i className="fa-solid fa-plus"></i><span>创建犬种</span>
        </button>
      </div>

      {loading && <div className="text-center text-slate-500 text-xs py-8">加载中...</div>}
      {error && <div className="text-center text-rose-400 text-xs py-8">加载失败</div>}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {breeds.map((p) => {
          const color = p.color?.primary ?? getBreedColor(p.id).primary;
          const icon = p.avatar ?? '';
          return (
            <div key={p.id} className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
              <div className="p-4 flex items-start justify-between">
                <div className="flex items-center space-x-3">
                  <div className="w-10 h-10 rounded-xl flex items-center justify-center text-white shadow shrink-0" style={{ backgroundColor: color }}>
                    <i className={icon}></i>
                  </div>
                  <div>
                    <div className="text-sm font-bold text-slate-100">{p.name}</div>
                    <div className="text-[11px] text-slate-400 font-mono">{p.display_name}</div>
                  </div>
                </div>
                <div className="flex items-center space-x-2">
                  <button onClick={() => setEditingBreed(p)} className="p-1.5 text-slate-500 hover:text-amber-400 transition" title="编辑犬种">
                    <i className="fa-solid fa-pen text-xs"></i>
                  </button>
                  <button
                    onClick={() => toggleEnabled(p.id, !p.enabled)}
                    className={clsx('w-11 h-6 rounded-full p-0.5 transition-colors relative focus:outline-none', p.enabled ? 'bg-amber-600' : 'bg-slate-800')}
                  >
                    <div className={clsx('w-5 h-5 rounded-full bg-white shadow-md transform transition-transform', p.enabled ? 'translate-x-5' : 'translate-x-0')}></div>
                  </button>
                </div>
              </div>

              <div className="px-4 pb-3 space-y-2">
                <div className="text-[11px] text-slate-400 font-mono">
                  <i className="fa-solid fa-microchip text-slate-500 mr-1"></i>{(p.variants?.[0]?.client_id) ?? '—'} · {(p.variants?.[0]?.default_model) ?? '—'}
                </div>
                <p className="text-[11px] text-slate-400 leading-relaxed">{p.personality}</p>
              </div>

              <button
                onClick={() => setExpandedId(expandedId === p.id ? null : p.id)}
                className="w-full px-4 py-2 border-t border-slate-800/80 text-[11px] text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 transition flex items-center justify-between"
              >
                <span>变体配置 ({p.variants?.length ?? 0})</span>
                <i className={clsx('fa-solid fa-chevron-down transition-transform', expandedId === p.id && 'rotate-180')}></i>
              </button>

              {expandedId === p.id && (
                <div className="px-4 py-3 border-t border-slate-800/80 bg-slate-950/40 space-y-2">
                  {(p.variants ?? []).map((v) => (
                    <div key={v.id} className="space-y-1">
                      <div className="flex items-center justify-between">
                        <span className="px-2 py-1 rounded-lg bg-slate-800 border border-slate-700 text-[10px] font-mono text-slate-300">{v.id}</span>
                        <button onClick={() => setEditingBreed(p)} className="text-[10px] text-amber-400 hover:text-amber-300">
                          <i className="fa-solid fa-pen"></i> 编辑
                        </button>
                      </div>
                      <div className="text-[10px] text-slate-500 font-mono pl-2">
                        {v.client_id} · {v.default_model} · mcp: {String(v.mcp_support)}
                      </div>
                      {v.cli?.command && (
                        <div className="text-[10px] text-slate-500 font-mono pl-2">cli: {v.cli.command} {v.cli.output_format}</div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {showCreate && <HubBreedEditor onSave={handleSaveBreed} onClose={() => setShowCreate(false)} />}
      {editingBreed && <HubBreedEditor breed={editingBreed} onSave={handleSaveBreed} onClose={() => setEditingBreed(null)} />}
    </div>
  );
}
