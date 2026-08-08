import { useCallback, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { settingsService } from '../../services/settingsService';
import { useAppStore } from '../../store/useAppStore';
import type { EnvSummary, EnvVariable } from '../../types';

const CATEGORY_ICONS: Record<string, string> = {
  model: 'fa-solid fa-brain',
  rag: 'fa-solid fa-database',
  runtime: 'fa-solid fa-server',
  workspace: 'fa-solid fa-folder',
};

type SaveStatus = 'idle' | 'saving' | 'success' | 'error';

export function ConfigPanel() {
  const [summary, setSummary] = useState<EnvSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle');
  const [saveMsg, setSaveMsg] = useState('');
  const showToast = useAppStore((s) => s.showToast);

  const fetchSummary = useCallback(async () => {
    setLoading(true);
    try {
      const data = await settingsService.getEnvSummary();
      setSummary(data);
      setDrafts({});
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `加载配置失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchSummary(); }, [fetchSummary]);

  // Build a flat map of all variables for dirty checking
  const allVars = useMemo(() => {
    const map: Record<string, EnvVariable> = {};
    if (!summary) return map;
    for (const cat of summary.categories) {
      for (const v of cat.variables) {
        map[v.key] = v;
      }
    }
    return map;
  }, [summary]);

  const dirtyKeys = useMemo(() => {
    return Object.keys(drafts).filter((k) => drafts[k] !== (allVars[k]?.value ?? ''));
  }, [drafts, allVars]);

  const isDirty = dirtyKeys.length > 0;

  const handleDraftChange = (key: string, value: string) => {
    setDrafts((prev) => ({ ...prev, [key]: value }));
    setSaveStatus('idle');
  };

  const handleSave = async () => {
    if (!isDirty) return;
    setSaveStatus('saving');
    setSaveMsg('');
    try {
      const updates = dirtyKeys.map((k) => ({ key: k, value: drafts[k] }));
      const updated = await settingsService.updateEnv(updates);
      setSaveStatus('success');
      setSaveMsg(`已更新 ${updated.length} 项配置`);
      showToast({ message: `配置已保存 (${updated.length} 项)`, type: 'success' });
      await fetchSummary();
      setTimeout(() => setSaveStatus('idle'), 3000);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setSaveStatus('error');
      setSaveMsg(msg);
      showToast({ message: `保存失败: ${msg}`, type: 'error' });
    }
  };

  const handleReset = () => {
    setDrafts({});
    setSaveStatus('idle');
  };

  const storageMode = summary?.storage_mode ?? 
    (summary?.categories.flatMap(c => c.variables).find(v => v.key === 'RAG_STORE_BACKEND')?.value ?? 'unknown');
  const isMemoryMode = storageMode === 'memory';

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5 flex items-start justify-between">
        <div>
          <h2 className="text-2xl font-bold text-slate-100">系统配置</h2>
          <p className="text-xs text-slate-400 mt-1">后端环境变量与运行时配置（可编辑）。</p>
        </div>
        <div className="flex items-center space-x-2">
          {isDirty && (
            <button onClick={handleReset} className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-semibold transition">
              重置
            </button>
          )}
          <button
            onClick={handleSave}
            disabled={!isDirty || saveStatus === 'saving'}
            className={clsx(
              'px-4 py-1.5 rounded-xl text-[11px] font-semibold transition',
              saveStatus === 'saving' && 'bg-amber-500/20 text-amber-300 border border-amber-500/30 animate-pulse',
              saveStatus === 'success' && 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30',
              saveStatus === 'error' && 'bg-rose-500/20 text-rose-300 border border-rose-500/30',
              saveStatus === 'idle' && isDirty && 'bg-indigo-500 text-white hover:bg-indigo-400',
              !isDirty && 'bg-slate-800 text-slate-500 cursor-not-allowed',
            )}
          >
            {saveStatus === 'saving' ? '保存中...' : saveStatus === 'success' ? '已保存' : saveStatus === 'error' ? '保存失败' : '保存'}
          </button>
        </div>
      </div>

      {saveMsg && (
        <div className={clsx('text-xs font-mono p-2 rounded-xl border', saveStatus === 'success' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30' : 'text-rose-400 bg-rose-500/10 border-rose-500/30')}>
          {saveMsg}
        </div>
      )}

      {/* Storage mode warning */}
      {isMemoryMode && (
        <div className="p-3 rounded-2xl bg-amber-500/10 border border-amber-500/30 flex items-start space-x-2">
          <i className="fa-solid fa-triangle-exclamation text-amber-400 text-xs mt-0.5"></i>
          <div>
            <div className="text-xs font-bold text-amber-300">存储模式: memory（非持久化）</div>
            <div className="text-[11px] text-amber-400/80 mt-0.5">RAG 数据存储在内存中，重启后丢失。建议设置 RAG_STORE_BACKEND=sqlite 以持久化。</div>
          </div>
        </div>
      )}

      {/* Data directories */}
      {summary?.data_dirs && Object.keys(summary.data_dirs).length > 0 && (
        <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
          <div className="flex items-center space-x-2 mb-2">
            <i className="fa-solid fa-folder-tree text-indigo-400 text-xs"></i>
            <h4 className="text-xs font-bold text-slate-200">数据目录</h4>
          </div>
          <div className="space-y-1.5">
            {Object.entries(summary.data_dirs).map(([key, path]) => (
              <div key={key} className="flex items-center justify-between text-[11px]">
                <span className="text-slate-500 font-mono">{key}</span>
                <span className="text-slate-300 font-mono">{path}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {loading && <div className="text-center text-slate-500 text-xs py-8">加载中...</div>}

      {/* Config grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {summary?.categories.map((cat) => (
          <div key={cat.name} className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
            <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
              <i className={clsx('fa-solid', CATEGORY_ICONS[cat.name] ?? 'fa-solid fa-sliders', 'text-amber-400 text-xs')}></i>
              <h4 className="text-xs font-bold text-slate-200 uppercase">{cat.name}</h4>
            </div>
            <div className="divide-y divide-slate-800/40">
              {cat.variables.map((item) => {
                const draftVal = drafts[item.key] ?? item.value;
                const isDirtyItem = drafts[item.key] !== undefined && drafts[item.key] !== item.value;
                const displayVal = item.sensitive && !isDirtyItem ? maskValue(item.value) : draftVal;
                return (
                  <div key={item.key} className={clsx('px-4 py-2.5', isDirtyItem && 'bg-indigo-500/5')}>
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="text-[11px] font-mono text-slate-200 font-semibold flex items-center gap-1.5">
                          {item.key}
                          {item.sensitive && <i className="fa-solid fa-lock text-[9px] text-amber-400"></i>}
                          {isDirtyItem && <span className="w-1.5 h-1.5 rounded-full bg-indigo-400"></span>}
                        </div>
                        {item.description && <div className="text-[10px] text-slate-500 mt-0.5">{item.description}</div>}
                      </div>
                      <input
                        type={item.sensitive ? 'password' : 'text'}
                        value={displayVal}
                        onChange={(e) => handleDraftChange(item.key, e.target.value)}
                        placeholder={item.sensitive ? '••••••••' : '(未设置)'}
                        className={clsx(
                          'text-[11px] font-mono ml-3 shrink-0 w-40 px-2 py-1 rounded-lg border bg-slate-800/50 transition',
                          isDirtyItem
                            ? 'border-indigo-500/50 text-indigo-200'
                            : 'border-slate-700/50 text-slate-400 focus:border-slate-600',
                        )}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function maskValue(val: string): string {
  if (!val || val === '••••••••') return val;
  if (val.length <= 4) return '****';
  return val.slice(0, 2) + '****' + val.slice(-2);
}
