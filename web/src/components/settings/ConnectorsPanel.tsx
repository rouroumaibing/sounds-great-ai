import { useCallback, useEffect, useState } from 'react';
import clsx from 'clsx';
import { useI18n } from '../../store/useI18n';
import {
  listConnectors,
  createConnector,
  updateConnector,
  deleteConnector,
  testConnector,
} from '../../services/panelsService';
import type { Connector, ConnectorType, ConnectorProbeResult } from '../../types/panels';
import {
  SettingsSection,
  SettingsText,
  SettingsBadge,
  SettingsSecondaryButton,
  SettingsStatusStrip,
} from './primitives';

const inputCls =
  'w-full rounded-[10px] border border-slate-800 bg-slate-950 px-3 py-1.5 text-compact leading-5 text-slate-200 placeholder:text-slate-600 outline-none transition focus:border-amber-500 focus:ring-1 focus:ring-amber-500/30';

const TYPES: ConnectorType[] = ['slack', 'telegram', 'webhook', 'generic'];

interface Draft {
  name: string;
  type: ConnectorType;
  endpoint: string;
  auth_key: string;
}

const emptyDraft: Draft = { name: '', type: 'webhook', endpoint: '', auth_key: '' };

// ConnectorsPanel (panels-roadmap P2): IM/webhook connector registry —
// CRUD + reachability probe. Auth keys are write-only (server masks them).
export function ConnectorsPanel() {
  const { t } = useI18n();
  const [items, setItems] = useState<Connector[]>([]);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [showDraft, setShowDraft] = useState(false);
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [probes, setProbes] = useState<Record<string, ConnectorProbeResult | 'testing'>>({});
  const [confirmingDelete, setConfirmingDelete] = useState('');

  const load = useCallback(async () => {
    try {
      setItems(await listConnectors());
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const submitDraft = async () => {
    if (!draft.name.trim() || !draft.endpoint.trim() || busy) return;
    setBusy('create');
    setError('');
    try {
      await createConnector({
        name: draft.name.trim(),
        type: draft.type,
        endpoint: draft.endpoint.trim(),
        auth_key: draft.auth_key || undefined,
      });
      setDraft(emptyDraft);
      setShowDraft(false);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  };

  const toggle = async (c: Connector) => {
    setBusy(`toggle:${c.id}`);
    setError('');
    try {
      await updateConnector(c.id, { enabled: !c.enabled });
      await load();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy('');
    }
  };

  const probe = async (c: Connector) => {
    setProbes((p) => ({ ...p, [c.id]: 'testing' }));
    setError('');
    try {
      const result = await testConnector(c.id);
      setProbes((p) => ({ ...p, [c.id]: result }));
    } catch (e) {
      setProbes((p) => ({ ...p, [c.id]: { ok: false, latency_ms: 0, status: 'error', error: String(e) } }));
    }
  };

  const remove = async (c: Connector) => {
    if (confirmingDelete !== c.id) {
      setConfirmingDelete(c.id);
      setTimeout(() => setConfirmingDelete((cur) => (cur === c.id ? '' : cur)), 3000);
      return;
    }
    setBusy(`delete:${c.id}`);
    setError('');
    try {
      await deleteConnector(c.id);
      await load();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy('');
      setConfirmingDelete('');
    }
  };

  const probeView = (c: Connector) => {
    const p = probes[c.id];
    if (p === 'testing') return <SettingsBadge tone="slate">{t('im.testing')}</SettingsBadge>;
    if (!p) return null;
    return p.ok ? (
      <SettingsBadge tone="emerald">{t('im.connected')} · {p.latency_ms}ms</SettingsBadge>
    ) : (
      <SettingsBadge tone="red">{t('im.notConnected')}</SettingsBadge>
    );
  };

  return (
    <div className="space-y-6">
      {error && <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip>}

      <SettingsSection title={t('im.title')} description={t('im.desc')}>
        <div className="mt-2 space-y-2">
          {items.length === 0 && !showDraft && (
            <SettingsText variant="xs" tone="muted">{t('im.empty')}</SettingsText>
          )}
          {items.map((c) => (
            <div key={c.id} className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2 min-w-0">
                  <SettingsBadge tone="blue">{c.type}</SettingsBadge>
                  <span className="text-sm font-medium text-slate-100 truncate">{c.name}</span>
                  <SettingsBadge tone={c.enabled ? 'emerald' : 'slate'}>
                    {c.enabled ? t('common.enabled') : t('common.disabled')}
                  </SettingsBadge>
                  {probeView(c)}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <button
                    onClick={() => toggle(c)}
                    disabled={busy === `toggle:${c.id}`}
                    className="px-2 py-0.5 rounded text-[10px] border border-slate-700 text-slate-300 hover:text-slate-100 transition disabled:opacity-40"
                  >
                    {c.enabled ? t('common.disable') : t('common.enable')}
                  </button>
                  <button
                    onClick={() => void probe(c)}
                    className="px-2 py-0.5 rounded text-[10px] bg-indigo-600/80 hover:bg-indigo-600 text-white transition"
                  >
                    {t('im.testConnect')}
                  </button>
                  <button
                    onClick={() => void remove(c)}
                    className={clsx(
                      'px-2 py-0.5 rounded text-[10px] transition',
                      confirmingDelete === c.id
                        ? 'bg-rose-600 text-white'
                        : 'border border-slate-700 text-slate-400 hover:text-rose-400'
                    )}
                  >
                    {confirmingDelete === c.id ? t('im.confirmDelete') : t('common.delete')}
                  </button>
                </div>
              </div>
              <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-slate-500 font-mono">
                <span className="truncate max-w-md">{c.endpoint}</span>
                {c.auth_key_set && (
                  <span className="text-slate-600">
                    {t('im.authKey')}: {c.auth_key_preview ?? '***'}
                  </span>
                )}
                {c.last_check && <span className="text-slate-600">· {c.last_check}</span>}
              </div>
              {(() => {
                const p = probes[c.id];
                if (p && p !== 'testing' && !p.ok && p.error) {
                  return <p className="mt-1 text-[11px] text-rose-400 truncate">{p.error}</p>;
                }
                return null;
              })()}
            </div>
          ))}
        </div>

        {showDraft ? (
          <div className="mt-3 rounded-xl border border-amber-500/30 bg-amber-500/5 p-3 space-y-2">
            <div className="grid gap-2 sm:grid-cols-2">
              <label className="space-y-1">
                <SettingsText variant="xs" tone="muted">{t('im.name')}</SettingsText>
                <input
                  value={draft.name}
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                  placeholder={t('im.inputPlaceholder', 'ops-webhook').replace('{label}', t('im.name'))}
                  maxLength={64}
                  className={inputCls}
                />
              </label>
              <label className="space-y-1">
                <SettingsText variant="xs" tone="muted">{t('im.type')}</SettingsText>
                <select
                  value={draft.type}
                  onChange={(e) => setDraft({ ...draft, type: e.target.value as ConnectorType })}
                  className={inputCls}
                >
                  {TYPES.map((ty) => (
                    <option key={ty} value={ty}>{ty}</option>
                  ))}
                </select>
              </label>
              <label className="space-y-1 sm:col-span-2">
                <SettingsText variant="xs" tone="muted">{t('im.endpoint')}</SettingsText>
                <input
                  value={draft.endpoint}
                  onChange={(e) => setDraft({ ...draft, endpoint: e.target.value })}
                  placeholder="https://hooks.example.com/…"
                  maxLength={512}
                  className={inputCls + ' font-mono'}
                />
              </label>
              <label className="space-y-1 sm:col-span-2">
                <SettingsText variant="xs" tone="muted">{t('im.authKey')}</SettingsText>
                <input
                  type="password"
                  value={draft.auth_key}
                  onChange={(e) => setDraft({ ...draft, auth_key: e.target.value })}
                  placeholder="••••••"
                  maxLength={512}
                  className={inputCls + ' font-mono'}
                />
              </label>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => void submitDraft()}
                disabled={!draft.name.trim() || !draft.endpoint.trim() || busy === 'create'}
                className="px-3 py-1 rounded-lg bg-amber-600 hover:bg-amber-500 text-white text-xs font-medium transition disabled:opacity-40"
              >
                {busy === 'create' ? '…' : t('common.add')}
              </button>
              <SettingsSecondaryButton onClick={() => { setShowDraft(false); setDraft(emptyDraft); }}>
                {t('common.cancel')}
              </SettingsSecondaryButton>
            </div>
          </div>
        ) : (
          <div className="mt-3">
            <SettingsSecondaryButton onClick={() => setShowDraft(true)}>
              <i className="fa-solid fa-plus mr-1 text-[10px]"></i>{t('im.add')}
            </SettingsSecondaryButton>
          </div>
        )}
      </SettingsSection>
    </div>
  );
}
