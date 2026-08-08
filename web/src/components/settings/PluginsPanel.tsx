import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet, apiPatch } from '../../services/http';

interface Plugin {
  id: string;
  name: string;
  version: string;
  status: 'active' | 'inactive';
  description: string;
  enabled: boolean;
}

export function PluginsPanel() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [installUrl, setInstallUrl] = useState('');
  const [configModal, setConfigModal] = useState<Plugin | null>(null);

  useEffect(() => {
    apiGet<Plugin[]>('/api/plugins').then((data) => setPlugins(Array.isArray(data) ? data : [])).catch(() => setPlugins([]));
  }, []);

  const togglePlugin = (id: string) => {
    setPlugins((prev) => prev.map((p) => p.id === id ? { ...p, enabled: !p.enabled, status: !p.enabled ? 'active' : 'inactive' } : p));
    const plugin = plugins.find((p) => p.id === id);
    if (plugin) apiPatch(`/api/plugins/${id}`, { enabled: !plugin.enabled }).catch(() => {});
  };

  const uninstall = (id: string) => {
    setPlugins((prev) => prev.filter((p) => p.id !== id));
  };

  const install = () => {
    if (!installUrl) return;
    const name = installUrl.split('/').pop()?.replace('.git', '') ?? 'new-plugin';
    setPlugins((prev) => [...prev, { id: `p${Date.now()}`, name, version: '0.1.0', status: 'active', description: `从 ${installUrl} 安装`, enabled: true }]);
    setInstallUrl('');
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">插件集成</h2>
        <p className="text-xs text-slate-400 mt-1">管理已安装插件，启用/禁用/安装/卸载。</p>
      </div>

      {/* Install from URL */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 p-4">
        <div className="flex items-center space-x-2">
          <input
            type="text"
            value={installUrl}
            onChange={(e) => setInstallUrl(e.target.value)}
            placeholder="https://github.com/example/plugin.git"
            className="flex-1 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] font-mono text-slate-200 focus:border-indigo-500/50 transition"
          />
          <button onClick={install} disabled={!installUrl} className="px-4 py-1.5 rounded-xl bg-indigo-500 text-white text-[11px] font-semibold hover:bg-indigo-400 transition disabled:opacity-50">
            <i className="fa-solid fa-download mr-1"></i>安装
          </button>
        </div>
      </div>

      {/* Plugin list */}
      <div className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800/80 flex items-center space-x-2">
          <i className="fa-solid fa-puzzle-piece text-amber-400 text-xs"></i>
          <h4 className="text-xs font-bold text-slate-200">已安装插件 ({plugins.length})</h4>
        </div>
        <div className="divide-y divide-slate-800/40">
          {plugins.map((p) => (
            <div key={p.id} className="px-4 py-3 flex items-center justify-between">
              <div className="flex items-center space-x-3 min-w-0">
                <div className={clsx('w-9 h-9 rounded-xl flex items-center justify-center shrink-0', p.enabled ? 'bg-indigo-500/20' : 'bg-slate-800')}>
                  <i className={clsx('fa-solid fa-puzzle-piece text-sm', p.enabled ? 'text-indigo-400' : 'text-slate-500')}></i>
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-slate-200">{p.name}</span>
                    <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">v{p.version}</span>
                    <span className={clsx('text-[10px] font-semibold', p.status === 'active' ? 'text-emerald-400' : 'text-slate-500')}>● {p.status}</span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-0.5 truncate">{p.description}</div>
                </div>
              </div>
              <div className="flex items-center space-x-2 shrink-0">
                <button onClick={() => setConfigModal(p)} className="px-2 py-1 rounded-lg bg-slate-800 text-slate-400 text-[11px] hover:bg-slate-700 transition">
                  <i className="fa-solid fa-gear"></i>
                </button>
                <button onClick={() => uninstall(p.id)} className="px-2 py-1 rounded-lg bg-slate-800 text-rose-400 text-[11px] hover:bg-rose-500/20 transition">
                  <i className="fa-solid fa-trash"></i>
                </button>
                <ToggleSwitch checked={p.enabled} onChange={() => togglePlugin(p.id)} />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Config modal */}
      {configModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => setConfigModal(null)}>
          <div className="bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl p-6 w-96 space-y-4" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-slate-100">{configModal.name} 配置</h3>
              <button onClick={() => setConfigModal(null)} className="text-slate-400 hover:text-slate-200"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <div className="space-y-2 text-[11px]">
              <div className="flex justify-between"><span className="text-slate-500">版本</span><span className="text-slate-200 font-mono">v{configModal.version}</span></div>
              <div className="flex justify-between"><span className="text-slate-500">状态</span><span className="text-slate-200">{configModal.status}</span></div>
              <div className="flex justify-between"><span className="text-slate-500">描述</span><span className="text-slate-300 text-right max-w-[200px]">{configModal.description}</span></div>
            </div>
            <pre className="text-[10px] font-mono text-slate-400 bg-slate-950/50 rounded-xl p-3 border border-slate-800">{`{\n  "enabled": ${configModal.enabled},\n  "auto_update": true\n}`}</pre>
          </div>
        </div>
      )}
    </div>
  );
}

function ToggleSwitch({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button onClick={onChange} className={clsx('relative w-10 h-5 rounded-full transition', checked ? 'bg-indigo-500' : 'bg-slate-700')}>
      <span className={clsx('absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform', checked ? 'translate-x-5' : 'translate-x-0.5')} />
    </button>
  );
}
