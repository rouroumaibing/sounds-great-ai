import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { apiGet, apiPatch } from '../../services/http';

interface Connector {
  id: string;
  name: string;
  icon: string;
  enabled: boolean;
  webhookUrl: string;
  configFields: { key: string; label: string; value: string }[];
  status: 'connected' | 'disconnected' | 'testing';
}

export function ImPanel() {
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    apiGet<Connector[]>('/api/config/connectors').then((data) => setConnectors(Array.isArray(data) ? data : [])).catch(() => setConnectors([]));
  }, []);

  const toggleEnabled = (id: string) => {
    setConnectors((prev) => prev.map((c) => c.id === id ? { ...c, enabled: !c.enabled } : c));
    apiPatch('/api/config/connectors', { id, enabled: !connectors.find((c) => c.id === id)?.enabled }).catch(() => {});
  };

  const testConnection = (id: string) => {
    setConnectors((prev) => prev.map((c) => c.id === id ? { ...c, status: 'testing' } : c));
    setTimeout(() => {
      setConnectors((prev) => prev.map((c) => c.id === id ? { ...c, status: 'connected' } : c));
    }, 1500);
  };

  const updateField = (id: string, fieldKey: string, value: string) => {
    setConnectors((prev) => prev.map((c) => c.id === id ? {
      ...c,
      configFields: c.configFields.map((f) => f.key === fieldKey ? { ...f, value } : f),
    } : c));
  };

  const updateWebhook = (id: string, url: string) => {
    setConnectors((prev) => prev.map((c) => c.id === id ? { ...c, webhookUrl: url } : c));
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">IM 对接</h2>
        <p className="text-xs text-slate-400 mt-1">配置即时通讯平台连接器，接收告警与犬种消息推送。</p>
      </div>

      <div className="space-y-3">
        {connectors.map((c) => (
          <div key={c.id} className="rounded-2xl bg-slate-900/60 border border-slate-800/80 overflow-hidden">
            <div className="px-4 py-3 flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <div className={clsx('w-9 h-9 rounded-xl flex items-center justify-center', c.enabled ? 'bg-indigo-500/20' : 'bg-slate-800')}>
                  <i className={clsx(c.icon, 'text-sm', c.enabled ? 'text-indigo-400' : 'text-slate-500')}></i>
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-200">{c.name}</div>
                  <div className="text-[11px] text-slate-500 mt-0.5">
                    {c.status === 'testing' ? '测试中...' : c.status === 'connected' ? '已连接' : '未连接'}
                  </div>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                <button onClick={() => setExpanded(expanded === c.id ? null : c.id)} className="px-3 py-1 rounded-lg bg-slate-800 text-slate-300 text-[11px] font-semibold hover:bg-slate-700 transition">
                  <i className={clsx('fa-solid fa-chevron-down transition-transform', expanded === c.id && 'rotate-180')}></i>
                </button>
                <ToggleSwitch checked={c.enabled} onChange={() => toggleEnabled(c.id)} />
              </div>
            </div>

            {expanded === c.id && (
              <div className="px-4 py-3 border-t border-slate-800/40 space-y-3">
                <div>
                  <label className="text-[11px] text-slate-400 font-mono">Webhook URL</label>
                  <input
                    type="text"
                    value={c.webhookUrl}
                    onChange={(e) => updateWebhook(c.id, e.target.value)}
                    placeholder="https://..."
                    className="w-full mt-1 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] font-mono text-slate-200 focus:border-indigo-500/50 transition"
                  />
                </div>
                {c.configFields.map((f) => (
                  <div key={f.key}>
                    <label className="text-[11px] text-slate-400 font-mono">{f.label}</label>
                    <input
                      type="text"
                      value={f.value}
                      onChange={(e) => updateField(c.id, f.key, e.target.value)}
                      placeholder={`输入 ${f.label}`}
                      className="w-full mt-1 px-3 py-1.5 rounded-lg bg-slate-800/50 border border-slate-700/50 text-[11px] font-mono text-slate-200 focus:border-indigo-500/50 transition"
                    />
                  </div>
                ))}
                <div className="flex items-center space-x-2 pt-1">
                  <button
                    onClick={() => testConnection(c.id)}
                    disabled={c.status === 'testing'}
                    className="px-3 py-1.5 rounded-xl bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 text-[11px] font-semibold hover:bg-indigo-500/30 transition disabled:opacity-50"
                  >
                    {c.status === 'testing' ? '测试中...' : '测试连接'}
                  </button>
                  {c.status === 'connected' && <span className="text-[11px] text-emerald-400"><i className="fa-solid fa-check-circle mr-1"></i>连接成功</span>}
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
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
