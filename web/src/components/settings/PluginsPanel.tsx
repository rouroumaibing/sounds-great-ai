import { useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import { useI18n } from '../../store/useI18n';
import {
  listPlugins,
  installPlugin,
  setPluginEnabled,
  uninstallPlugin,
} from '../../services/pluginsService';
import {
  browseMarketplace,
  installFromMarketplace,
  type MarketplaceItem,
} from '../../services/pluginsService';
import type { PluginView } from '../../types/plugins';
import {
  SettingsSection,
  SettingsText,
  SettingsBadge,
  SettingsStatusStrip,
  SettingsSecondaryButton,
} from './primitives';

// PluginsPanel (panels-roadmap P3): install (zip upload, lands disabled with
// skills in pending review), enable/disable (enable gates on approved
// skills; breeds register through validation on enable), uninstall.
export function PluginsPanel() {
  const { t } = useI18n();
  const [items, setItems] = useState<PluginView[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState('');
  const [confirmingDelete, setConfirmingDelete] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);
  const [mktQuery, setMktQuery] = useState('');
  const [mktItems, setMktItems] = useState<MarketplaceItem[] | null>(null);
  const [mktError, setMktError] = useState('');
  const [mktBusy, setMktBusy] = useState('');

  const load = useCallback(async () => {
    try {
      setItems(await listPlugins());
    } catch (e) {
      setError(String(e));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const onUpload = async (file: File | undefined) => {
    if (!file) return;
    setBusy('install');
    setError('');
    setNotice('');
    try {
      const result = await installPlugin(file);
      setNotice(
        `${result.plugin.name} v${result.plugin.version} ${t('plugins.installedDisabled')}` +
          (result.notes.length ? ` — ${result.notes.join('；')}` : ''),
      );
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  const toggle = async (p: PluginView) => {
    setBusy(`toggle:${p.id}`);
    setError('');
    setNotice('');
    try {
      await setPluginEnabled(p.id, !p.enabled);
      await load();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      // 409: skills pending/quarantined block the enable — point at the
      // skills panel's security review.
      setError(`${t('plugins.enableBlocked')}：${msg}`);
    } finally {
      setBusy('');
    }
  };

  const remove = async (p: PluginView) => {
    if (confirmingDelete !== p.id) {
      setConfirmingDelete(p.id);
      setTimeout(() => setConfirmingDelete((cur) => (cur === p.id ? '' : cur)), 3000);
      return;
    }
    setBusy(`delete:${p.id}`);
    setError('');
    try {
      await uninstallPlugin(p.id);
      setConfirmingDelete('');
      await load();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy('');
    }
  };

  const searchMarketplace = async () => {
    setMktBusy('search');
    setMktError('');
    try {
      const res = await browseMarketplace(mktQuery.trim());
      setMktItems(res.plugins);
      if (res.note) setMktError(res.note);
    } catch (e) {
      setMktError(e instanceof Error ? e.message : String(e));
    } finally {
      setMktBusy('');
    }
  };

  const installFromMarket = async (id: string) => {
    setMktBusy(id);
    setError('');
    setNotice('');
    try {
      const result = await installFromMarketplace(id);
      setNotice(`${result.plugin.name} v${result.plugin.version} ${t('plugins.installedDisabled')}`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setMktBusy('');
    }
  };

  return (
    <div className="space-y-6">
      {error && <SettingsStatusStrip tone="error">{error}</SettingsStatusStrip>}
      {notice && <SettingsStatusStrip tone="success">{notice}</SettingsStatusStrip>}

      <SettingsSection title={t('plugins.title')} description={t('plugins.desc')}>
        <div className="mt-2 flex items-center gap-2 flex-wrap">
          <input
            ref={fileRef}
            type="file"
            accept=".zip"
            className="hidden"
            onChange={(e) => void onUpload(e.target.files?.[0])}
          />
          <SettingsSecondaryButton onClick={() => fileRef.current?.click()} disabled={busy === 'install'}>
            <i className="fa-solid fa-box-open mr-1 text-[10px]"></i>
            {busy === 'install' ? t('plugins.installing') : t('plugins.upload')}
          </SettingsSecondaryButton>
          <SettingsText variant="micro" tone="muted">{t('plugins.uploadHint')}</SettingsText>
        </div>

        <div className="mt-3 space-y-2">
          <SettingsText variant="xs" tone="muted">
            {t('plugins.installed').replace('{count}', String(items.length))}
          </SettingsText>
          {items.map((p) => (
            <div key={p.id} className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2 min-w-0">
                  <SettingsBadge tone={p.enabled ? 'emerald' : 'slate'}>
                    {p.enabled ? t('common.enabled') : t('common.disabled')}
                  </SettingsBadge>
                  <span className="text-sm font-medium text-slate-100 truncate">{p.name}</span>
                  <span className="font-mono text-[10px] text-slate-500">{p.id}@{p.version}</span>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <button
                    onClick={() => void toggle(p)}
                    disabled={busy === `toggle:${p.id}`}
                    className="px-2 py-0.5 rounded text-[10px] border border-slate-700 text-slate-300 hover:text-slate-100 transition disabled:opacity-40"
                  >
                    {p.enabled ? t('common.disable') : t('common.enable')}
                  </button>
                  <button
                    onClick={() => void remove(p)}
                    className={clsx(
                      'px-2 py-0.5 rounded text-[10px] transition',
                      confirmingDelete === p.id
                        ? 'bg-rose-600 text-white'
                        : 'border border-slate-700 text-slate-400 hover:text-rose-400',
                    )}
                  >
                    {confirmingDelete === p.id ? t('im.confirmDelete') : t('common.delete')}
                  </button>
                </div>
              </div>
              {p.description && (
                <p className="mt-1 text-[11px] text-slate-400 leading-snug">{p.description}</p>
              )}
              <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-slate-500 font-mono">
                {p.skills.length > 0 && (
                  <span className="text-amber-400/80">skills: {p.skills.join(', ')}</span>
                )}
                {p.breeds.length > 0 && <span>breeds: {p.breeds.join(', ')}</span>}
                {p.permissions && p.permissions.length > 0 && (
                  <span className="text-slate-600">permissions: {p.permissions.join(', ')}</span>
                )}
                <span className="text-slate-600">{new Date(p.installed_at).toLocaleString()}</span>
              </div>
              {!p.enabled && p.skills.length > 0 && (
                <p className="mt-1 text-[10px] text-amber-400/70">{t('plugins.reviewHint')}</p>
              )}
            </div>
          ))}
          {items.length === 0 && (
            <SettingsText variant="xs" tone="muted">{t('plugins.empty')}</SettingsText>
          )}
        </div>
      </SettingsSection>

      <SettingsSection title={t('marketplace.title')} description={t('marketplace.desc')}>
        <div className="mt-2 space-y-2">
          <div className="flex gap-1.5">
            <input
              value={mktQuery}
              onChange={(e) => setMktQuery(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void searchMarketplace(); }}
              placeholder={t('marketplace.searchPlaceholder')}
              className="flex-1 rounded-[10px] border border-slate-800 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 placeholder:text-slate-600 outline-none focus:border-indigo-500"
            />
            <SettingsSecondaryButton onClick={() => void searchMarketplace()} disabled={mktBusy === 'search'}>
              <i className="fa-solid fa-magnifying-glass mr-1 text-[10px]"></i>{t('common.search')}
            </SettingsSecondaryButton>
          </div>

          {mktError && <SettingsStatusStrip tone="warn">{mktError}</SettingsStatusStrip>}

          {mktItems !== null && (
            <div className="space-y-2">
              {mktItems.length === 0 ? (
                <SettingsText variant="xs" tone="muted">{t('marketplace.notFound')}</SettingsText>
              ) : (
                mktItems.map((it) => {
                  const installedIds = items.map((p) => p.id);
                  return (
                    <div key={`${it.id}@${it.version}`} className="rounded-xl border border-slate-800/80 bg-slate-900/60 p-3">
                      <div className="flex items-center justify-between gap-2 flex-wrap">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="text-sm font-medium text-slate-100 truncate">{it.name}</span>
                          <span className="font-mono text-[10px] text-slate-500">{it.id}@{it.version}</span>
                          {it.publisher && <SettingsBadge tone="blue">{it.publisher}</SettingsBadge>}
                          {installedIds.includes(it.id) ? (
                            <SettingsBadge tone="emerald">{t('marketplace.installedShort')}</SettingsBadge>
                          ) : null}
                        </div>
                        <button
                          onClick={() => void installFromMarket(it.id)}
                          disabled={installedIds.includes(it.id) || mktBusy === it.id}
                          className="px-2.5 py-1 rounded-lg bg-indigo-600/90 hover:bg-indigo-600 text-white text-[10px] font-medium transition disabled:opacity-40"
                        >
                          {mktBusy === it.id
                            ? t('plugins.installing')
                            : installedIds.includes(it.id)
                              ? t('marketplace.alreadyInstalled')
                              : t('plugins.installAction')}
                        </button>
                      </div>
                      {it.description && (
                        <p className="mt-1 text-[11px] text-slate-400 leading-snug">{it.description}</p>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          )}
          <SettingsText variant="micro" tone="muted">{t('marketplace.verifyNote')}</SettingsText>
        </div>
      </SettingsSection>
    </div>
  );
}
