import clsx from 'clsx';
import { useMemo, useState } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useI18n } from '../../store/useI18n';
import { useSettings } from '../../hooks/useSettings';
import { settingsService } from '../../services/settingsService';
import { AccountEditModal } from './AccountEditModal';
import type { SettingsAccount } from '../../types';

// --- helpers ---

function accountMeta(acc: SettingsAccount, t: (key: string) => string): string {
  const parts: string[] = [];
  if (acc.type === 'oauth') {
    if (acc.clientId) parts.push(acc.clientId);
  } else {
    const host = acc.baseUrl?.replace(/^https?:\/\//, '').replace(/\/+$/, '') || null;
    if (host) parts.push(host);
    parts.push(acc.hasApiKey ? t('accounts.configured') : t('accounts.notConfigured'));
  }
  if (acc.models && acc.models.length > 0) {
    parts.push(`${acc.models.length} ${t('accounts.modelCount')}`);
  } else {
    parts.push(t('accounts.noModels'));
  }
  return parts.join(' · ');
}

// --- sub-components ---

function AccountRow({
  acc,
  onEdit,
  onDelete,
}: {
  acc: SettingsAccount;
  onEdit: (acc: SettingsAccount) => void;
  onDelete: (id: string) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800/80 hover:border-slate-700 transition group">
      <div className="flex items-center justify-between gap-3">
        {/* left: identity */}
        <div className="flex items-center gap-3 min-w-0">
          <div className={clsx(
            'w-9 h-9 rounded-xl flex items-center justify-center shrink-0',
            acc.builtin ? 'bg-indigo-500/20 border border-indigo-500/30' : 'bg-slate-800 border border-slate-700',
          )}>
            <i className={clsx('fa-solid fa-key text-sm', acc.builtin ? 'text-indigo-300' : 'text-slate-400')} />
          </div>
          <div className="min-w-0 space-y-0.5">
            <div className="flex items-center gap-2">
              <span className="text-sm font-bold text-slate-100 font-mono truncate">{acc.displayName ?? acc.name}</span>
              {acc.builtin ? (
                <span className="px-1.5 py-0.5 rounded-lg bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 text-[9px] font-mono">{t('accounts.builtinTag')}</span>
              ) : (
                <span className="px-1.5 py-0.5 rounded-lg bg-slate-800 text-slate-400 border border-slate-700 text-[9px] font-mono">{t('accounts.customTag')}</span>
              )}
              {acc.hasApiKey && (
                <span className="text-emerald-400 text-[10px]" title={t('accounts.apiKeySet')}><i className="fa-solid fa-check-circle" /></span>
              )}
            </div>
            <div className="text-[11px] font-mono text-slate-400 truncate">{accountMeta(acc, t)}</div>
          </div>
        </div>

        {/* right: badges + actions */}
        <div className="flex items-center gap-2 shrink-0">
          <span className={clsx(
            'px-2 py-0.5 rounded-lg text-[10px] font-mono font-bold uppercase border',
            acc.type === 'oauth' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-purple-500/20 text-purple-300 border-purple-500/40',
          )}>{acc.type}</span>
          {/* model tags */}
          {acc.models && acc.models.length > 0 && (
            <div className="hidden md:flex items-center gap-1">
              {acc.models.slice(0, 3).map((m) => (
                <span key={m} className="px-1.5 py-0.5 rounded-lg bg-slate-800 border border-slate-700 text-[9px] font-mono text-slate-300">{m}</span>
              ))}
              {acc.models.length > 3 && <span className="text-[9px] text-slate-500">+{acc.models.length - 3}</span>}
            </div>
          )}
          <button onClick={() => onEdit(acc)} className="p-1.5 text-slate-500 hover:text-amber-400 transition" title={t('accounts.editAccount')}>
            <i className="fa-solid fa-pen text-xs"></i>
          </button>
          <button onClick={() => onDelete(acc.id)} className="p-1.5 text-slate-500 hover:text-rose-400 transition" title={t('accounts.deleteAccount')}>
            <i className="fa-regular fa-trash-can text-xs"></i>
          </button>
        </div>
      </div>
    </div>
  );
}

// --- main ---

export function AccountKeys() {
  const { t } = useI18n();
  const showAddAccountModal = useAppStore((s) => s.showAddAccountModal);
  const setShowAddAccountModal = useAppStore((s) => s.setShowAddAccountModal);
  const showToast = useAppStore((s) => s.showToast);
  const { accounts, loading, addAccount, deleteAccount, refetch } = useSettings();

  const [editingAccount, setEditingAccount] = useState<SettingsAccount | null>(null);

  const dispatchAccountsChanged = () => {
    window.dispatchEvent(new CustomEvent('accounts-changed'));
  };

  const handleSaveAccount = async (form: Omit<SettingsAccount, 'id'> & { id?: string; apiKey?: string }) => {
    try {
      if (form.id) {
        await settingsService.updateAccount(form.id, form);
      } else {
        await addAccount(form.name, form.clientId ?? form.name, form.apiKey ?? '');
      }
      dispatchAccountsChanged();
      await refetch();
      setShowAddAccountModal(false);
      setEditingAccount(null);
      showToast({ message: t('accounts.saved'), type: 'success' });
    } catch {
      showToast({ message: t('accounts.saveFailed'), type: 'error' });
    }
  };

  const handleDeleteAccount = async (id: string) => {
    try {
      await deleteAccount(id);
      dispatchAccountsChanged();
      showToast({ message: t('accounts.deleted'), type: 'success' });
    } catch {
      // error handled in hook
    }
  };

  // separate builtin and custom
  const { builtinAccounts, customAccounts } = useMemo(() => {
    const builtin = accounts.filter((a) => a.builtin);
    const custom = accounts.filter((a) => !a.builtin);
    return { builtinAccounts: builtin, customAccounts: custom };
  }, [accounts]);

  return (
    <div className="max-w-5xl mx-auto w-full space-y-4">
      {/* action bar */}
      <div className="flex items-center justify-end">
        <button onClick={() => setShowAddAccountModal(true)} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-lg shadow-amber-600/20">
          <i className="fa-solid fa-plus"></i><span>{t('accounts.create')}</span>
        </button>
      </div>

      {/* loading */}
      {loading && <div className="text-center text-slate-500 text-xs py-8">{t('common.loading')}</div>}

      {/* empty state */}
      {!loading && accounts.length === 0 && (
        <div className="text-center py-12">
          <i className="fa-solid fa-key text-3xl text-slate-700 mb-3"></i>
          <p className="text-sm text-slate-500">{t('accounts.empty')}</p>
          <p className="text-xs text-slate-600 mt-1">{t('accounts.emptyHint')}</p>
        </div>
      )}

      {/* builtin accounts */}
      {!loading && builtinAccounts.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs text-slate-400 font-medium">
            <i className="fa-solid fa-shield-halved"></i>
            <span>{t('accounts.builtin')}</span>
            <span className="text-slate-600">({builtinAccounts.length})</span>
          </div>
          {builtinAccounts.map((acc) => (
            <AccountRow key={acc.id} acc={acc} onEdit={setEditingAccount} onDelete={handleDeleteAccount} />
          ))}
        </div>
      )}

      {/* custom accounts */}
      {!loading && customAccounts.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs text-slate-400 font-medium">
            <i className="fa-solid fa-user-pen"></i>
            <span>{t('accounts.custom')}</span>
            <span className="text-slate-600">({customAccounts.length})</span>
          </div>
          {customAccounts.map((acc) => (
            <AccountRow key={acc.id} acc={acc} onEdit={setEditingAccount} onDelete={handleDeleteAccount} />
          ))}
        </div>
      )}

      {/* status strip */}
      {!loading && accounts.length > 0 && (
        <div className="text-[11px] text-slate-500 text-center pt-2 border-t border-slate-800/60">
          <i className="fa-solid fa-circle-info mr-1"></i>
          {t('accounts.statusStrip')}
        </div>
      )}

      {/* modals */}
      {showAddAccountModal && <AccountEditModal onSave={handleSaveAccount} onClose={() => setShowAddAccountModal(false)} />}
      {editingAccount && <AccountEditModal account={editingAccount} onSave={handleSaveAccount} onClose={() => setEditingAccount(null)} />}
    </div>
  );
}
