import clsx from 'clsx';
import { useEffect, useState } from 'react';
import { apiGet, ApiError } from '../../services/http';

type McpServer = { name: string; tools: string[]; enabled: boolean };

export function McpPanel() {
  const [mcpServers, setMcpServers] = useState<McpServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState<Record<string, boolean>>({});
  const [reconnecting, setReconnecting] = useState<Set<string>>(new Set());

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    apiGet<McpServer[]>('/api/mcp/servers')
      .then((data) => {
        if (cancelled) return;
        setMcpServers(data);
        setConnected(Object.fromEntries(data.map((s) => [s.name, s.enabled])));
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : '加载失败');
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleReconnect = (name: string) => {
    setReconnecting((prev) => new Set(prev).add(name));
    setTimeout(() => {
      setReconnecting((prev) => {
        const next = new Set(prev);
        next.delete(name);
        return next;
      });
      setConnected((prev) => ({ ...prev, [name]: true }));
    }, 1500);
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="border-b border-slate-800/80 pb-5">
        <h2 className="text-2xl font-bold text-slate-100">MCP 管理</h2>
        <p className="text-xs text-slate-400 mt-1">MCP 服务器连接状态与工具注册情况。</p>
      </div>

      {loading ? (
        <div className="text-center py-12 text-slate-400 text-sm">加载中...</div>
      ) : error ? (
        <div className="text-center py-12 text-rose-400 text-sm">{error}</div>
      ) : (
        <div className="space-y-3">
          {mcpServers.map((server) => {
            const isConnected = connected[server.name];
            const isReconnecting = reconnecting.has(server.name);
            return (
              <div key={server.name} className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800/80">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center space-x-3">
                    <div className={clsx('w-2.5 h-2.5 rounded-full', isReconnecting ? 'bg-amber-400 animate-pulse' : isConnected ? 'bg-emerald-500' : 'bg-rose-500')}></div>
                    <div>
                      <span className="text-sm font-bold text-slate-100 font-mono">{server.name}</span>
                      <span className="text-[11px] text-slate-400 ml-2">{server.tools.length} tools</span>
                    </div>
                  </div>
                  <button
                    onClick={() => handleReconnect(server.name)}
                    disabled={isReconnecting}
                    className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] font-semibold flex items-center gap-1.5 transition disabled:opacity-50"
                  >
                    <i className={clsx('fa-solid', isReconnecting ? 'fa-spinner animate-spin' : 'fa-rotate')}></i>
                    <span>{isReconnecting ? '连接中...' : '重新连接'}</span>
                  </button>
                </div>
                <div className="flex flex-wrap gap-2">
                  {server.tools.map((tool) => (
                    <span key={tool} className="px-2 py-1 rounded-lg bg-slate-800 border border-slate-700 text-[10px] font-mono text-slate-300">
                      <i className="fa-solid fa-wrench text-slate-500 mr-1"></i>{tool}
                    </span>
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
