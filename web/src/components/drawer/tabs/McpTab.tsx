import { useEffect, useState } from 'react';
import { apiGet, apiPost, apiPut, apiDelete } from '../../../services/http';
import { useRagBackend } from '../../../hooks/useRagBackend';
import clsx from 'clsx';
import type { LoadedSkill, McpServer, McpFallback } from '../../../types';
import { useI18n } from '../../../store/useI18n';

// Parse a KEY=VALUE textarea into a map. Lines whose value is exactly "***"
// (the masked sentinel returned by the API) are skipped so we never clobber a
// real secret the operator did not intend to change.
function parseKV(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx < 0) continue;
    const k = trimmed.slice(0, idx).trim();
    const v = trimmed.slice(idx + 1).trim();
    if (k === '' || v === '***') continue;
    out[k] = v;
  }
  return out;
}

function kvToText(kv?: Record<string, string>): string {
  if (!kv) return '';
  return Object.keys(kv)
    .map((k) => `${k}=***`)
    .join('\n');
}

function statusBadge(s: McpServer): { label: string; cls: string } {
  switch (s.status) {
    case 'ok':
      return { label: 'mcp.status.online', cls: 'bg-emerald-500/20 text-emerald-400' };
    case 'empty':
      return { label: 'mcp.status.empty', cls: 'bg-amber-500/20 text-amber-400' };
    case 'error':
      return { label: 'mcp.status.error', cls: 'bg-rose-500/20 text-rose-400' };
    default:
      return { label: 'mcp.status.unknown', cls: 'bg-slate-500/20 text-slate-400' };
  }
}

// Known builtin servers get a human-readable capability description so the
// operator understands what each platform-provided MCP surface exposes.
const builtinDescKey: Record<string, string> = {
  knowledge: 'mcp.builtin.knowledge',
  platform: 'mcp.builtin.platform',
};

export function McpTab() {
  const { t } = useI18n();
  const [loadedSkills, setLoadedSkills] = useState<LoadedSkill[]>([]);
  const [mcpServers, setMcpServers] = useState<McpServer[]>([]);
  const [loadingServers, setLoadingServers] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { backend, loading, error: ragError, syncProgress, switching, syncing, switchBackend, triggerSync } = useRagBackend();

  const loadServers = (refresh = false) => {
    setLoadingServers(true);
    setError(null);
    apiGet<McpServer[]>(`/api/mcp/servers${refresh ? '?refresh=1' : ''}`)
      .then((data) => setMcpServers(Array.isArray(data) ? data : []))
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoadingServers(false));
  };

  useEffect(() => {
    apiGet<LoadedSkill[]>('/api/skills')
      .then((data) => setLoadedSkills(Array.isArray(data) ? data : []))
      .catch(() => setLoadedSkills([]));
    loadServers();
  }, []);

  const toggleServer = async (s: McpServer) => {
    setBusy(s.name);
    try {
      await apiPut(`/api/mcp/servers/${encodeURIComponent(s.name)}`, { enabled: !s.enabled });
      loadServers();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  const deleteServer = async (s: McpServer) => {
    if (!window.confirm(t('mcp.confirmDelete'))) return;
    setBusy(s.name);
    try {
      await apiDelete(`/api/mcp/servers/${encodeURIComponent(s.name)}`);
      loadServers();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  const saveEdit = async (s: McpServer, form: ServerFormState) => {
    setBusy(s.name);
    try {
      const patch: Record<string, unknown> = {
        breeds: form.breeds.split(',').map((b) => b.trim()).filter(Boolean),
      };
      if (form.transport === 'remote') {
        patch.url = form.url;
        patch.headers = parseKV(form.headers);
      } else {
        patch.command = form.command;
        patch.args = form.args.split(/\s+/).filter(Boolean);
        patch.env = parseKV(form.env);
      }
      await apiPut(`/api/mcp/servers/${encodeURIComponent(s.name)}`, patch);
      setEditing(null);
      loadServers();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-3">
      {/* RAG Backend Section */}
      <div>
        <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400 block mb-2">RAG Backend</span>
        {loading && <div className="text-center text-slate-500 text-xs py-2">{t('common.loading')}</div>}
        {ragError && <div className="text-center text-rose-400 text-xs py-2">{t('common.error')}: {ragError}</div>}
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
                {switching ? t('mcp.switching') : t('mcp.switchTo') + ' ' + (backend.active === 'memory' ? 'sqlite' : 'memory')}
              </button>
              <button
                onClick={() => triggerSync(backend.active === 'memory' ? 'sqlite' : 'memory')}
                disabled={syncing}
                className={clsx('px-2 py-1 rounded-lg text-[10px] font-mono transition', syncing ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-amber-600 hover:bg-amber-500 text-white')}
              >
                {syncing ? t('mcp.syncing') : t('mcp.syncData')}
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

      {/* MCP Servers Management Section */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="font-bold text-[11px] uppercase tracking-wider text-slate-400">{t('mcp.servers')}</span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => loadServers(true)}
              disabled={loadingServers}
              className={clsx('px-1.5 py-0.5 rounded text-[9px] font-mono transition', loadingServers ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-slate-700 hover:bg-slate-600 text-slate-200')}
            >
              {t('mcp.refresh')}
            </button>
            <button
              onClick={() => setShowAdd((v) => !v)}
              className="px-1.5 py-0.5 rounded text-[9px] font-mono bg-cyan-600 hover:bg-cyan-500 text-white"
            >
              {showAdd ? t('mcp.cancel') : '+ ' + t('mcp.add')}
            </button>
          </div>
        </div>

        {error && <div className="text-center text-rose-400 text-[10px] py-1">{t('mcp.errorPrefix')}{error}</div>}
        {loadingServers && <div className="text-center text-slate-500 text-xs py-2">{t('common.loading')}</div>}

        {showAdd && <AddServerForm onAdd={async (body) => {
          setBusy('__add__');
          try {
            await apiPost('/api/mcp/servers', body);
            setShowAdd(false);
            loadServers();
          } catch (e) {
            setError(e instanceof Error ? e.message : String(e));
          } finally {
            setBusy(null);
          }
        }} busy={busy === '__add__'} t={t} />}

        <div className="space-y-1.5">
          {mcpServers.map((s) => {
            const badge = statusBadge(s);
            const isEditing = editing === s.name;
            return (
              <ServerCard
                key={s.name}
                s={s}
                badge={badge}
                isEditing={isEditing}
                busy={busy === s.name}
                t={t}
                onToggle={() => toggleServer(s)}
                onDelete={() => deleteServer(s)}
                onStartEdit={() => setEditing(s.name)}
                onCancelEdit={() => setEditing(null)}
                onSave={(form) => saveEdit(s, form)}
              />
            );
          })}
        </div>
        <p className="text-[9px] text-slate-500 leading-snug mt-1">{t('mcp.addHint')}</p>
      </div>
    </div>
  );
}

interface ServerFormState {
  transport: 'stdio' | 'remote';
  command: string;
  args: string;
  env: string;
  url: string;
  headers: string;
  breeds: string;
}

// ServerCard renders one MCP server with its tools, a transport badge, an
// optional HTTP-callback (fallback) disclosure, and the edit form.
function ServerCard({
  s, t, badge, isEditing, busy, onToggle, onDelete, onStartEdit, onCancelEdit, onSave,
}: {
  s: McpServer;
  t: (k: string) => string;
  badge: { label: string; cls: string };
  isEditing: boolean;
  busy: boolean;
  onToggle: () => void;
  onDelete: () => void;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  onSave: (form: ServerFormState) => void;
}) {
  const [showFallback, setShowFallback] = useState(false);
  const [fallback, setFallback] = useState<McpFallback | null>(null);
  const [fallbackLoading, setFallbackLoading] = useState(false);

  const toggleFallback = () => {
    if (showFallback) {
      setShowFallback(false);
      return;
    }
    setShowFallback(true);
    setFallbackLoading(true);
    apiGet<McpFallback>(`/api/mcp/servers/${encodeURIComponent(s.name)}/fallback`)
      .then((data) => setFallback(data))
      .catch(() => setFallback({ name: s.name, note: 'failed to load fallback' }))
      .finally(() => setFallbackLoading(false));
  };

  const isRemote = !!s.url;

  return (
    <div className="p-2 rounded-lg bg-slate-950 border border-slate-800 space-y-1">
      <div className="flex items-center justify-between">
        <span className="font-mono text-[11px] text-cyan-300 font-bold">{s.display_name || s.name}</span>
        <div className="flex items-center gap-1">
          {isRemote && <span className="text-[9px] px-1.5 py-0.5 rounded bg-sky-500/20 text-sky-300 font-mono">{t('mcp.remote')}</span>}
          <span className={clsx('text-[9px] px-1.5 py-0.5 rounded font-mono', badge.cls)}>{t(badge.label)}</span>
          {s.builtin && <span className="text-[9px] px-1.5 py-0.5 rounded bg-slate-700/50 text-slate-400 font-mono">{t('mcp.builtin')}</span>}
        </div>
      </div>
      {s.builtin && builtinDescKey[s.name] && (
        <div className="text-[9px] text-slate-500 leading-snug">{t(builtinDescKey[s.name])}</div>
      )}
      <div className="text-[10px] text-slate-500 font-mono break-all">
        {isRemote ? s.url : `${s.command || ''}${s.args && s.args.length ? ' ' + s.args.join(' ') : ''}`}
      </div>
      <div className="text-[10px] text-slate-400 font-mono">
        {t('mcp.tools')}: {s.tools && s.tools.length ? s.tools.join(', ') : t('mcp.noTools')}
      </div>
      {s.error && <div className="text-[9px] text-rose-400 font-mono">{s.error}</div>}

      {/* HTTP callback (fallback) disclosure */}
      {((s.fallback_available || s.callback_url) && !isEditing) && (
        <div className="pt-0.5">
          <button onClick={toggleFallback} className="text-[9px] text-sky-300 font-mono hover:underline">
            {showFallback ? t('mcp.hideFallback') : t('mcp.showFallback')}
          </button>
          {showFallback && (
            <div className="mt-1 p-1.5 rounded bg-slate-900/80 border border-slate-800 space-y-1">
              {fallbackLoading && <div className="text-[9px] text-slate-500">{t('common.loading')}</div>}
              {!fallbackLoading && fallback && (
                <>
                  <div className="text-[9px] text-slate-400">{t('mcp.fallbackHint')}</div>
                  {fallback.callback_url && (
                    <div className="text-[9px] text-slate-500 font-mono break-all">{t('mcp.callbackUrl')}: {fallback.callback_url}</div>
                  )}
                  {fallback.tools && fallback.tools.length > 0 ? (
                    <div className="space-y-1 max-h-40 overflow-auto">
                      {fallback.tools.map((tool) => (
                        <div key={tool.name} className="text-[9px] font-mono">
                          <div className="text-slate-300">{tool.method} {tool.path} <span className="text-slate-500">({tool.name})</span></div>
                          <pre className="text-[8px] text-slate-500 whitespace-pre-wrap break-all mt-0.5">{tool.sample}</pre>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-[9px] text-slate-500">{fallback.note || t('mcp.fallbackEmpty')}</div>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      )}

      {!isEditing ? (
        <div className="flex items-center gap-2 pt-0.5">
          <button
            onClick={onToggle}
            disabled={busy}
            className={clsx('px-2 py-0.5 rounded text-[9px] font-mono transition', busy ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : (s.enabled ? 'bg-emerald-600 hover:bg-emerald-500 text-white' : 'bg-slate-700 hover:bg-slate-600 text-slate-200'))}
          >
            {s.enabled ? 'ON' : 'OFF'}
          </button>
          {!s.builtin && (
            <>
              <button onClick={onStartEdit} className="px-2 py-0.5 rounded text-[9px] font-mono bg-slate-700 hover:bg-slate-600 text-slate-200">{t('mcp.edit')}</button>
              <button onClick={onDelete} disabled={busy} className="px-2 py-0.5 rounded text-[9px] font-mono bg-rose-700 hover:bg-rose-600 text-white">{t('mcp.delete')}</button>
            </>
          )}
        </div>
      ) : (
        <EditServerForm
          s={s}
          onSave={onSave}
          onCancel={onCancelEdit}
          busy={busy}
          t={t}
        />
      )}
    </div>
  );
}

function AddServerForm({ onAdd, busy, t }: { onAdd: (body: McpServer) => void; busy: boolean; t: (k: string) => string }) {
  const [name, setName] = useState('');
  const [transport, setTransport] = useState<'stdio' | 'remote'>('stdio');
  const [command, setCommand] = useState('');
  const [args, setArgs] = useState('');
  const [env, setEnv] = useState('');
  const [url, setUrl] = useState('');
  const [headers, setHeaders] = useState('');
  const [breeds, setBreeds] = useState('');

  const body: McpServer = {
    name,
    tools: [],
    enabled: true,
    breeds: breeds.split(',').map((b) => b.trim()).filter(Boolean),
  };
  if (transport === 'remote') {
    body.url = url;
    body.headers = parseKV(headers);
  } else {
    body.command = command;
    body.args = args.split(/\s+/).filter(Boolean);
    body.env = parseKV(env);
  }

  const canSubmit = name.length > 0 && (transport === 'remote' ? url.length > 0 : command.length > 0);

  return (
    <div className="p-2 rounded-lg bg-slate-900 border border-slate-700 space-y-1.5">
      <div className="grid grid-cols-2 gap-1.5">
        <label className="text-[9px] text-slate-400">{t('mcp.serverName')}<input value={name} onChange={(e) => setName(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
        <label className="text-[9px] text-slate-400">{t('mcp.breeds')}<input value={breeds} onChange={(e) => setBreeds(e.target.value)} placeholder="bianmu,jinmao" className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
      </div>
      <TransportToggle transport={transport} setTransport={setTransport} t={t} />
      {transport === 'remote' ? (
        <>
          <label className="block text-[9px] text-slate-400">{t('mcp.url')}<input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/mcp" className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.headers')}<textarea value={headers} onChange={(e) => setHeaders(e.target.value)} rows={2} placeholder="Authorization=Bearer xxx" className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
        </>
      ) : (
        <>
          <label className="block text-[9px] text-slate-400">{t('mcp.command')}<input value={command} onChange={(e) => setCommand(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.args')}<input value={args} onChange={(e) => setArgs(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.env')}<textarea value={env} onChange={(e) => setEnv(e.target.value)} rows={2} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
        </>
      )}
      <button
        onClick={() => onAdd(body)}
        disabled={busy || !canSubmit}
        className={clsx('w-full px-2 py-1 rounded text-[10px] font-mono', busy || !canSubmit ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-cyan-600 hover:bg-cyan-500 text-white')}
      >
        {t('mcp.add')}
      </button>
    </div>
  );
}

function EditServerForm({ s, onSave, onCancel, busy, t }: { s: McpServer; onSave: (form: ServerFormState) => void; onCancel: () => void; busy: boolean; t: (k: string) => string }) {
  const [transport, setTransport] = useState<'stdio' | 'remote'>(s.url ? 'remote' : 'stdio');
  const [command, setCommand] = useState(s.command || '');
  const [args, setArgs] = useState((s.args || []).join(' '));
  const [env, setEnv] = useState(kvToText(s.env));
  const [url, setUrl] = useState(s.url || '');
  const [headers, setHeaders] = useState(kvToText(s.headers));
  const [breeds, setBreeds] = useState((s.breeds || []).join(', '));

  return (
    <div className="p-2 rounded-lg bg-slate-900 border border-slate-700 space-y-1.5">
      <TransportToggle transport={transport} setTransport={setTransport} t={t} />
      {transport === 'remote' ? (
        <>
          <label className="block text-[9px] text-slate-400">{t('mcp.url')}<input value={url} onChange={(e) => setUrl(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.headers')}<textarea value={headers} onChange={(e) => setHeaders(e.target.value)} rows={2} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
        </>
      ) : (
        <>
          <label className="block text-[9px] text-slate-400">{t('mcp.command')}<input value={command} onChange={(e) => setCommand(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.args')}<input value={args} onChange={(e) => setArgs(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
          <label className="block text-[9px] text-slate-400">{t('mcp.env')}<textarea value={env} onChange={(e) => setEnv(e.target.value)} rows={2} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
        </>
      )}
      <label className="block text-[9px] text-slate-400">{t('mcp.breeds')}<input value={breeds} onChange={(e) => setBreeds(e.target.value)} className="w-full mt-0.5 bg-slate-950 border border-slate-700 rounded px-1 py-0.5 text-[10px] text-slate-200 font-mono" /></label>
      <div className="flex items-center gap-2">
        <button onClick={() => onSave({ transport, command, args, env, url, headers, breeds })} disabled={busy} className={clsx('px-2 py-0.5 rounded text-[9px] font-mono', busy ? 'bg-slate-800 text-slate-500 cursor-not-allowed' : 'bg-emerald-600 hover:bg-emerald-500 text-white')}>{t('mcp.save')}</button>
        <button onClick={onCancel} className="px-2 py-0.5 rounded text-[9px] font-mono bg-slate-700 hover:bg-slate-600 text-slate-200">{t('mcp.cancel')}</button>
      </div>
    </div>
  );
}

function TransportToggle({ transport, setTransport, t }: { transport: 'stdio' | 'remote'; setTransport: (v: 'stdio' | 'remote') => void; t: (k: string) => string }) {
  return (
    <div className="flex items-center gap-1">
      <span className="text-[9px] text-slate-400">{t('mcp.transport')}:</span>
      <button onClick={() => setTransport('stdio')} className={clsx('px-1.5 py-0.5 rounded text-[9px] font-mono', transport === 'stdio' ? 'bg-cyan-600 text-white' : 'bg-slate-800 text-slate-400')}>{t('mcp.transport.stdio')}</button>
      <button onClick={() => setTransport('remote')} className={clsx('px-1.5 py-0.5 rounded text-[9px] font-mono', transport === 'remote' ? 'bg-cyan-600 text-white' : 'bg-slate-800 text-slate-400')}>{t('mcp.transport.remote')}</button>
    </div>
  );
}
