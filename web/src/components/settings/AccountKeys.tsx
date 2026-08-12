import { useMemo, useState } from 'react';
import { useI18n } from '../../store/useI18n';
import { useSettings } from '../../hooks/useSettings';
import { useAppStore } from '../../store/useAppStore';
import { AccountAuthModal, type UnifiedAuthEditData } from './AccountAuthModal';
import { SettingsBadge, SettingsEmptyState, SettingsIconButton, SettingsPrimaryButton, SettingsRow, SettingsStatusStrip } from './primitives';
import type { SettingsAccount } from '../../types';
import { ApiError } from '../../services/http';

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

// --- main ---

export function AccountKeys() {
  const { t } = useI18n();
  const showToast = useAppStore((s) => s.showToast);
  const { accounts, loading, deleteAccount, refetch } = useSettings();

  const [authModalOpen, setAuthModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<UnifiedAuthEditData | undefined>(undefined);

  const dispatchAccountsChanged = () => {
    window.dispatchEvent(new CustomEvent('accounts-changed'));
  };

  const openCreate = () => {
    setEditTarget(undefined);
    setAuthModalOpen(true);
  };

  const openEdit = (acc: SettingsAccount) => {
    setEditTarget({
      id: acc.id,
      displayName: acc.displayName,
      baseUrl: acc.baseUrl,
      clientId: acc.clientId as UnifiedAuthEditData['clientId'],
      authType: acc.authType,
      models: acc.models,
      envVars: acc.envVars,
    });
    setAuthModalOpen(true);
  };

  const handleAuthCreated = async () => {
    setAuthModalOpen(false);
    setEditTarget(undefined);
    dispatchAccountsChanged();
    await refetch();
    showToast({ message: t('accounts.saved'), type: 'success' });
  };

  const handleDeleteAccount = async (id: string) => {
    try {
      await deleteAccount(id);
      dispatchAccountsChanged();
      showToast({ message: t('accounts.deleted'), type: 'success' });
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        // Account is referenced by a member — confirm a forced delete.
        if (window.confirm(t('accounts.confirmDeleteBound'))) {
          try {
            await deleteAccount(id, true);
            dispatchAccountsChanged();
            showToast({ message: t('accounts.deleted'), type: 'success' });
          } catch {
            showToast({ message: t('accounts.deleteFailed'), type: 'error' });
          }
        }
      } else {
        showToast({ message: t('accounts.deleteFailed'), type: 'error' });
      }
    }
  };

  // separate builtin and custom
  const { builtinAccounts, customAccounts } = useMemo(() => {
    const builtin = accounts.filter((a) => a.builtin);
    const custom = accounts.filter((a) => !a.builtin);
    return { builtinAccounts: builtin, customAccounts: custom };
  }, [accounts]);

  return (
    <div className="space-y-4">
      {/* 右上角新增账户认证 */}
      <div className="flex items-center justify-end">
        <SettingsPrimaryButton onClick={openCreate}>
          <i className="fa-solid fa-plus mr-1" />
          {t('accounts.create')}
        </SettingsPrimaryButton>
      </div>

      {loading && <SettingsStatusStrip tone="muted">{t('common.loading')}</SettingsStatusStrip>}

      {!loading && accounts.length === 0 && (
        <SettingsEmptyState
          icon={<i className="fa-solid fa-key text-4xl text-slate-600" />}
          title={t('accounts.empty')}
          description={t('accounts.emptyHint')}
        />
      )}

      {/* 内置账户组 + 自定义账户组，资源行卡片 */}
      {!loading && builtinAccounts.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs text-slate-400 font-medium">
            <i className="fa-solid fa-shield-halved" />
            <span>{t('accounts.builtin')}</span>
            <span className="text-slate-600">({builtinAccounts.length})</span>
          </div>
          {builtinAccounts.map((acc) => (
            <SettingsRow
              key={acc.id}
              icon={
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-indigo-500/20">
                  <i className="fa-solid fa-key text-sm text-indigo-300" />
                </div>
              }
              title={<span className="font-mono">{acc.displayName ?? acc.name}</span>}
              meta={<span className="font-mono">{accountMeta(acc, t)}</span>}
              badges={
                <>
                  <SettingsBadge tone="blue" size="xxs">{t('accounts.builtinTag')}</SettingsBadge>
                  {acc.hasApiKey && <SettingsBadge tone="emerald" size="xxs">已配密钥</SettingsBadge>}
                </>
              }
              actions={
                <>
                  <SettingsIconButton title={t('accounts.editAccount')} onClick={() => openEdit(acc)}>
                    <i className="fa-solid fa-pen text-xs" />
                  </SettingsIconButton>
                  <SettingsIconButton tone="danger" title={t('accounts.deleteAccount')} onClick={() => handleDeleteAccount(acc.id)}>
                    <i className="fa-regular fa-trash-can text-xs" />
                  </SettingsIconButton>
                </>
              }
            />
          ))}
        </div>
      )}

      {!loading && customAccounts.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-xs text-slate-400 font-medium">
            <i className="fa-solid fa-user-pen" />
            <span>{t('accounts.custom')}</span>
            <span className="text-slate-600">({customAccounts.length})</span>
          </div>
          {customAccounts.map((acc) => (
            <SettingsRow
              key={acc.id}
              icon={
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-slate-800">
                  <i className="fa-solid fa-key text-sm text-slate-400" />
                </div>
              }
              title={<span className="font-mono">{acc.displayName ?? acc.name}</span>}
              meta={<span className="font-mono">{accountMeta(acc, t)}</span>}
              badges={
                <>
                  <SettingsBadge tone={acc.type === 'oauth' ? 'amber' : 'purple'} size="xxs">{acc.type}</SettingsBadge>
                  {acc.hasApiKey && <SettingsBadge tone="emerald" size="xxs">已配密钥</SettingsBadge>}
                </>
              }
              actions={
                <>
                  <SettingsIconButton title={t('accounts.editAccount')} onClick={() => openEdit(acc)}>
                    <i className="fa-solid fa-pen text-xs" />
                  </SettingsIconButton>
                  <SettingsIconButton tone="danger" title={t('accounts.deleteAccount')} onClick={() => handleDeleteAccount(acc.id)}>
                    <i className="fa-regular fa-trash-can text-xs" />
                  </SettingsIconButton>
                </>
              }
            />
          ))}
        </div>
      )}

      {!loading && accounts.length > 0 && (
        <SettingsStatusStrip tone="muted">
          <i className="fa-solid fa-circle-info mr-1" />
          {t('accounts.statusStrip')}
        </SettingsStatusStrip>
      )}

      {/* 新增/编辑账户认证弹窗 */}
      <AccountAuthModal
        open={authModalOpen}
        onClose={() => {
          setAuthModalOpen(false);
          setEditTarget(undefined);
        }}
        onCreated={handleAuthCreated}
        editProfile={editTarget}
      />
    </div>
  );
}
