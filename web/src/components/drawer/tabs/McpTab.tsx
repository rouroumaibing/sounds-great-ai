import { useEffect, useState } from 'react';
import { apiGet } from '../../../services/http';
import { useRagBackend } from '../../../hooks/useRagBackend';
import clsx from 'clsx';
import type { LoadedSkill, McpServer } from '../../../types';

export function McpTab() {
  const [loadedSkills, setLoadedSkills] = useState<LoadedSkill[]>([]);
  const [mcpServers, setMcpServers] = useState<McpServer[]>([]);
  const { backend, loading, error, syncProgress, switching, syncing, switchBackend, triggerSync } = useRagBackend();

  useEffect(() => {
    apiGet<LoadedSkill[]>('/api/skills').then((data) => setLoadedSkills(Array.isArray(data) ? data : [])).catch(() => setLoadedSkills([]));
    apiGet<McpServer[]>('/api/mcp/servers').then((data) => setMcpServers(Array.isArray(data) ? data : [])).catch(() => setMcpServers([]));
  }, []);

  return (
    <div className="space-y-3">
      {/* RAG Backend Section */}
      <div>
        <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block mb-2">RAG Backend</span>
        {loading && <div className="text-center text-slate-500 text-xs py-2">加载中...</div>}
        {error && <div className="text-center text-rose-400 text-xs py-2">加载失败: {error}</div>}
        {backend && (
          <div className="space-y-2">
            <div className="p-2 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <i className="fa-solid fa-database text-emerald-400 text-[11px]"></i>
                <span className="font-mono text-[11px] text-slate-200">Active: {backend.active}</span>
              </div>
              <span className="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-mono">ACTIVE</span>
            </div>
            {backend.retirees.length > 0 && (
              <div className="space-y-1">
                {backend.retirees.map((r) => (
                  <div key={r.id} className="p-1.5 rounded-lg bg-slate-950/60 border border-slate-800/60 flex items-center justify-between">
                    <span className="font-mono text-[10px] text-slate-400">{r.id}</span>
                    <span className="text-[9px] text-slate-500 font-mono">{r.reason}</span>
                  </div>
                ))}
              </div>
            )}
            <div className="flex items-center gap-2">
              <button
                onClick={() => switchBackend(backend.active === 'memory' ? 'sqlite' : 'memory')}
                disabled={switching}
                className={clsx('px-2 py-1 rounded-lg text-[10px] font-mono transition', switching ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-indigo-600 hover:bg-indigo-500 text-white')}
              >
                {switching ? '切换中...' : `切换到 ${backend.active === 'memory' ? 'sqlite' : 'memory'}`}
              </button>
              <button
                onClick={() => triggerSync(backend.active === 'memory' ? 'sqlite' : 'memory')}
                disabled={syncing}
                className={clsx('px-2 py-1 rounded-lg text-[10px] font-mono transition', syncing ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-amber-600 hover:bg-amber-500 text-white')}
              >
                {syncing ? '同步中...' : '同步数据'}
              </button>
            </div>
            {syncProgress && (
              <div className="space-y-1">
                <div className="w-full h-1.5 rounded-full bg-slate-800 overflow-hidden">
                  <div
                    className={clsx('h-full rounded-full transition-all', syncProgress.status === 'error' ? 'bg-rose-500' : 'bg-emerald-500')}
                    style={{ width: `${syncProgress.total > 0 ? Math.min((syncProgress.current / syncProgress.total) * 100, 100) : 0}%` }}
                  />
                </div>
                <div className="text-[9px] text-slate-500 font-mono text-center">
                  {syncProgress.current}/{syncProgress.total} · {syncProgress.status}
                  {syncProgress.error && ` · ${syncProgress.error}`}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Loaded Skills Section */}
      <div>
        <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block mb-2">Loaded Dynamic Skills</span>
        <div className="space-y-1.5">
          {loadedSkills.map((skill) => (
            <div key={skill.name} className="p-2 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-between">
              <div className="flex items-center space-x-2"><i className="fa-solid fa-code text-indigo-400 text-[11px]"></i><span className="font-mono text-[11px] text-slate-200">{skill.name}</span></div>
              <span className="text-[10px] text-slate-500 font-mono">{skill.source}</span>
            </div>
          ))}
        </div>
      </div>

      {/* MCP Servers Section */}
      <div>
        <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block mb-2">MCP Servers Connected</span>
        <div className="space-y-1.5">
          {mcpServers.map((mcp) => (
            <div key={mcp.name} className="p-2 rounded-lg bg-slate-950 border border-slate-800 space-y-1">
              <div className="flex items-center justify-between"><span className="font-mono text-[11px] text-cyan-300 font-bold">{mcp.name}</span><span className="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-mono">ONLINE</span></div>
              <div className="text-[10px] text-slate-400 font-mono">Tools: {mcp.tools.join(', ')}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
