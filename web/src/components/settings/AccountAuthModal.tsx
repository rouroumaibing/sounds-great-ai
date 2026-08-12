import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { settingsService } from '../../services/settingsService';
import { CLIENT_IDS } from '../../constants/clientIds';
import { TagEditor } from './TagEditor';

type ClientId = (typeof CLIENT_IDS)[number]['id'];

/** Client dropdown options — derived from SG's CLIENT_IDS. */
const CLIENT_OPTIONS: ClientId[] = CLIENT_IDS.map((c) => c.id);

/** Suggested models per client — kept in sync with dog-template.json clientDefaults. */
const MODEL_SUGGESTIONS: Partial<Record<ClientId, string[]>> = {
  claude: [
    'claude-sonnet-4-6',
    'claude-opus-4-6',
    'claude-opus-4-6[1m]',
    'claude-sonnet-4-5-20250929',
    'claude-opus-4-5-20251101',
  ],
  codex: ['gpt-5.4', 'gpt-5.3-codex', 'gpt-5.3-codex-spark'],
  gemini: ['Gemini 3.1 Pro (High)', 'Gemini 3.1 Pro (Low)', 'Gemini 3.5 Flash (High)'],
  opencode: ['claude-sonnet-4-6', 'claude-opus-4-6'],
};

function clientLabel(id?: ClientId | string): string {
  const found = CLIENT_IDS.find((c) => c.id === id);
  return found?.label ?? (typeof id === 'string' ? id : 'Builtin');
}

/** System-reserved env var prefix — user-defined env injection must not
 *  clobber runtime variables (mirrors the backend filter in file_store.go). */
const RESERVED_ENV_PREFIX = 'SOUNDS_GREAT_AI_';

export interface UnifiedAuthEditData {
  id: string;
  displayName?: string;
  baseUrl?: string;
  clientId?: ClientId;
  authType?: string;
  models?: string[];
  envVars?: Record<string, string>;
}

type AuthMode = 'oauth' | 'api_key';

interface AccountAuthModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (profileId: string) => void;
  editProfile?: UnifiedAuthEditData;
  /** When provided, locks client to this value (wizard context). */
  initialClientId?: ClientId;
}

export function AccountAuthModal({ open, onClose, onCreated, editProfile, initialClientId }: AccountAuthModalProps) {
  const isEdit = Boolean(editProfile);
  const defaultClientId = editProfile?.clientId ?? initialClientId ?? 'claude';
  const [authMode, setAuthMode] = useState<AuthMode>(editProfile?.authType === 'api_key' ? 'api_key' : 'oauth');
  const [clientId, setClientId] = useState<ClientId>(defaultClientId);
  const [displayName, setDisplayName] = useState(editProfile?.displayName ?? '');
  const [baseUrl, setBaseUrl] = useState(editProfile?.baseUrl ?? '');
  const [apiKey, setApiKey] = useState('');
  const [models, setModels] = useState<string[]>(editProfile?.models ?? []);
  const [envEntries, setEnvEntries] = useState<Array<{ key: string; value: string }>>(
    editProfile?.envVars ? Object.entries(editProfile.envVars).map(([key, value]) => ({ key, value })) : [],
  );
  const [advancedOpen, setAdvancedOpen] = useState(
    Boolean(editProfile?.envVars && Object.keys(editProfile.envVars).length > 0),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Rehydrate form state when modal re-opens (same key but stale data)
  const prevOpenRef = useRef(open);
  useEffect(() => {
    if (open && !prevOpenRef.current) {
      const cid = editProfile?.clientId ?? initialClientId ?? 'claude';
      setClientId(cid);
      setAuthMode(editProfile?.authType === 'api_key' ? 'api_key' : 'oauth');
      setDisplayName(editProfile?.displayName ?? '');
      setBaseUrl(editProfile?.baseUrl ?? '');
      setModels(editProfile?.models ?? []);
      setApiKey('');
      setError(null);
      setEnvEntries(
        editProfile?.envVars ? Object.entries(editProfile.envVars).map(([key, value]) => ({ key, value })) : [],
      );
      setAdvancedOpen(Boolean(editProfile?.envVars && Object.keys(editProfile.envVars).length > 0));
    }
    prevOpenRef.current = open;
  }, [open, editProfile, initialClientId]);

  if (!open) return null;

  const isOAuth = authMode === 'oauth';

  /** POSIX env var key: must start with uppercase or _, rest alphanumeric + _.
   *  System-reserved prefix is rejected to avoid clobbering runtime vars. */
  const ENV_KEY_RE = /^[A-Z_][A-Za-z0-9_]*$/;
  const isValidEnvKey = (k: string) => ENV_KEY_RE.test(k) && !k.startsWith(RESERVED_ENV_PREFIX);

  /** Build envVars Record from entries, filtering empty/invalid/reserved keys. */
  const buildEnvVars = (): Record<string, string> | undefined => {
    const vars: Record<string, string> = {};
    for (const { key, value } of envEntries) {
      const k = key.trim();
      if (!k || !isValidEnvKey(k)) continue;
      vars[k] = value;
    }
    return Object.keys(vars).length > 0 ? vars : undefined;
  };

  const resetForm = () => {
    setClientId(defaultClientId);
    setAuthMode('oauth');
    setDisplayName('');
    setBaseUrl('');
    setApiKey('');
    setModels([]);
    setEnvEntries([]);
    setAdvancedOpen(false);
    setError(null);
  };

  const handleClose = () => {
    resetForm();
    onClose();
  };

  const canSubmit = isOAuth
    ? Boolean(displayName.trim())
    : Boolean(displayName.trim()) && models.length > 0 && (isEdit || Boolean(baseUrl.trim() && apiKey.trim()));

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSaving(true);
    setError(null);
    try {
      if (isEdit) {
        const envVars = buildEnvVars();
        const patch: Partial<{
          displayName: string;
          models: string[];
          envVars: Record<string, string>;
          clientId?: ClientId;
          baseUrl?: string;
          apiKey?: string;
        }> = {
          displayName: displayName.trim(),
          models,
          envVars: envVars ?? {},
        };
        if (editProfile?.clientId) {
          patch.clientId = clientId;
        }
        if (baseUrl.trim()) patch.baseUrl = baseUrl.trim();
        if (apiKey.trim()) patch.apiKey = apiKey.trim();
        await settingsService.updateAccount(editProfile!.id, patch, apiKey.trim() ? apiKey.trim() : undefined);
        onCreated(editProfile!.id);
        onClose();
      } else if (isOAuth) {
        const effectiveClientId = initialClientId ?? clientId;
        const created = await settingsService.addAccountFull({
          name: displayName.trim(),
          details: '',
          type: 'oauth',
          displayName: displayName.trim(),
          clientId: effectiveClientId,
          authType: 'oauth',
          mode: 'subscription',
          models,
          envVars: buildEnvVars() ?? {},
          builtin: false,
        });
        if (created.id) {
          resetForm();
          onCreated(created.id);
          onClose();
        }
      } else {
        const created = await settingsService.addAccountFull({
          name: displayName.trim(),
          details: '',
          type: 'api_key',
          displayName: displayName.trim(),
          clientId: initialClientId ?? clientId,
          authType: 'api_key',
          mode: 'api_key',
          baseUrl: baseUrl.trim(),
          models,
          envVars: buildEnvVars() ?? {},
          builtin: false,
        }, apiKey.trim() ? apiKey.trim() : undefined);
        if (created.id) {
          resetForm();
          onCreated(created.id);
          onClose();
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return createPortal(
    <div
      className="fixed inset-0 z-[80] flex items-center justify-center bg-black/60 px-4 backdrop-blur-sm"
      onClick={handleClose}
    >
      <div
        className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="mb-1 flex justify-end">
          <button
            type="button"
            onClick={handleClose}
            className="rounded-full p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200"
            aria-label="关闭"
          >
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <p className="mb-1 text-xs text-slate-500">{isEdit ? '编辑账户' : '系统配置 > 账户配置 > 添加认证'}</p>
        <h4 className="mb-4 text-base font-semibold text-slate-100">{isEdit ? '编辑账户认证' : '添加账户认证'}</h4>

        {/* Mode toggle */}
        <div
          className={`mb-4 flex rounded-lg border border-slate-800 p-0.5 ${isEdit ? 'opacity-50' : ''}`}
        >
          <button
            type="button"
            onClick={() => !isEdit && setAuthMode('oauth')}
            className={`flex-1 rounded-md py-1.5 text-xs font-medium transition ${
              isOAuth ? 'bg-amber-600 text-white shadow-sm' : 'text-slate-400'
            } ${isEdit ? 'cursor-not-allowed' : !isOAuth ? 'hover:bg-slate-800' : ''}`}
            disabled={isEdit}
          >
            OAuth
          </button>
          <button
            type="button"
            onClick={() => !isEdit && setAuthMode('api_key')}
            className={`flex-1 rounded-md py-1.5 text-xs font-medium transition ${
              !isOAuth ? 'bg-amber-600 text-white shadow-sm' : 'text-slate-400'
            } ${isEdit ? 'cursor-not-allowed' : isOAuth ? 'hover:bg-slate-800' : ''}`}
            disabled={isEdit}
          >
            API Key
          </button>
        </div>

        <div className="space-y-3" data-guide-id="accounts.create-details">
          {/* 账号名称 — always shown */}
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-300">账号名称</label>
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="例如: my-claude-account"
              className={formInputClass}
            />
          </div>

          {/* OAuth mode: Client dropdown */}
          {isOAuth && (
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-300">Client</label>
              {initialClientId ? (
                <p className={formInputClass}>{clientLabel(initialClientId)}</p>
              ) : (
                <select
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value as ClientId)}
                  className={formInputClass}
                >
                  {CLIENT_OPTIONS.map((c) => (
                    <option key={c} value={c}>
                      {clientLabel(c)}
                    </option>
                  ))}
                </select>
              )}
            </div>
          )}

          {/* API Key mode: Base URL + API Key */}
          {!isOAuth && (
            <>
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-300">
                  API 服务地址 (Base URL)
                </label>
                <input
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="https://api.openai.com/v1"
                  className={formInputClass}
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-300">
                  API Key{isEdit && '（留空保持不变）'}
                </label>
                <input
                  type="password"
                  autoComplete="off"
                  value={apiKey}
                  onChange={(e) => {
                    setApiKey(e.target.value);
                    setError(null);
                  }}
                  placeholder={isEdit ? '••••••••••••' : 'sk-...'}
                  className={formInputClass}
                />
              </div>
            </>
          )}

          {/* 可用模型 */}
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-300">可用模型</label>
            <TagEditor
              tags={models}
              onChange={setModels}
              placeholder="输入模型名"
            />
            {/* Model suggestions for builtin clients */}
            {isOAuth &&
              (MODEL_SUGGESTIONS[initialClientId ?? clientId] ?? []).filter((m) => !models.includes(m)).length > 0 && (
                <div className="mt-1.5 flex flex-wrap items-center gap-1">
                  <span className="text-[11px] text-slate-500">推荐</span>
                  {(MODEL_SUGGESTIONS[initialClientId ?? clientId] ?? [])
                    .filter((m) => !models.includes(m))
                    .map((m) => (
                      <button
                        key={m}
                        type="button"
                        onClick={() => setModels([...models, m])}
                        className="rounded-full border border-dashed border-slate-700 px-2 py-0.5 text-[11px] text-slate-400 transition hover:border-amber-500 hover:text-amber-400"
                      >
                        + {m}
                      </button>
                    ))}
                </div>
              )}
          </div>

          {/* 高级配置 — collapsible env var injection */}
          <div className="rounded-lg border border-slate-800">
            <button
              type="button"
              onClick={() => setAdvancedOpen((v) => !v)}
              className="flex w-full items-center gap-1 px-3 py-2 text-xs font-medium text-slate-300 hover:bg-slate-800"
            >
              <span className="text-[11px]">{advancedOpen ? '▾' : '▸'}</span>
              高级配置 (可选)
            </button>
            {advancedOpen && (
              <div className="border-t border-slate-800 px-3 pb-3 pt-2">
                <p className="mb-2 text-[11px] text-slate-500">
                  自定义环境变量，启动 agent 时注入子进程
                </p>
                <div className="space-y-1.5">
                  {envEntries.map((entry, i) => (
                    <div key={i} className="flex items-center gap-1.5">
                      <div className="w-[38%] shrink-0">
                        <input
                          value={entry.key}
                          onChange={(e) => {
                            const next = [...envEntries];
                            next[i] = { ...next[i], key: e.target.value };
                            setEnvEntries(next);
                          }}
                          placeholder="KEY"
                          className={`font-mono ${
                            entry.key.trim() && !isValidEnvKey(entry.key.trim())
                              ? `${formInputClass} !border-rose-500 !bg-rose-950/40 !text-rose-300`
                              : formInputClass
                          }`}
                          title="变量名须以大写字母或下划线开头，仅含 A-Z、0-9、_，且不能以 SOUNDS_GREAT_AI_ 开头"
                        />
                      </div>
                      <span className="text-[11px] text-slate-500">=</span>
                      <div className="min-w-0 flex-1">
                        <input
                          value={entry.value}
                          onChange={(e) => {
                            const next = [...envEntries];
                            next[i] = { ...next[i], value: e.target.value };
                            setEnvEntries(next);
                          }}
                          placeholder="value"
                          className={`font-mono ${formInputClass}`}
                        />
                      </div>
                      <button
                        type="button"
                        onClick={() => setEnvEntries(envEntries.filter((_, j) => j !== i))}
                        className="text-xs text-slate-500 hover:text-rose-400"
                        title="删除"
                      >
                        <TrashIcon className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  ))}
                  {envEntries.some((e) => e.key.trim() && !isValidEnvKey(e.key.trim())) && (
                    <p className="text-[11px] text-rose-400">
                      变量名须以大写字母或下划线开头，仅含 A-Z、0-9、_，且不能以 SOUNDS_GREAT_AI_ 开头
                    </p>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => setEnvEntries([...envEntries, { key: '', value: '' }])}
                  className="mt-2 text-[11px] font-medium text-amber-500 hover:text-amber-400"
                >
                  + 添加变量
                </button>
              </div>
            )}
          </div>
        </div>

        {error && <p className="mt-3 text-xs text-rose-400">{error}</p>}

        {/* Save button — bottom right */}
        <div className="mt-4 flex justify-end">
          <button
            type="button"
            data-guide-id="accounts.create-submit"
            onClick={handleSubmit}
            disabled={saving || !canSubmit}
            className="rounded-lg bg-amber-600 px-5 py-2 text-sm font-semibold text-white transition hover:bg-amber-500 disabled:opacity-50"
          >
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

const formInputClass =
  'h-9 w-full rounded-xl border border-slate-800 bg-slate-950 px-3 text-xs text-slate-200 outline-none placeholder:text-slate-500 transition focus:border-amber-500 focus:ring-1 focus:ring-amber-500';

function TrashIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className ?? 'h-3.5 w-3.5'}
      aria-hidden="true"
    >
      <path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M10 11v6M14 11v6" />
    </svg>
  );
}
