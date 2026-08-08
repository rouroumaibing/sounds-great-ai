import clsx from 'clsx';
import { useState } from 'react';
import { useAppStore } from '../../store/useAppStore';
import { useSettings } from '../../hooks/useSettings';
import { settingsService } from '../../services/settingsService';
import { AccountEditModal } from './AccountEditModal';
import type { SettingsAccount } from '../../types';

export function AccountKeys() {
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
        if (form.clientId || form.displayName || form.models || form.modelAliases || form.envVars || form.baseUrl) {
          // Extended fields need a separate update after creation
        }
      }
      dispatchAccountsChanged();
      await refetch();
      setShowAddAccountModal(false);
      setEditingAccount(null);
      showToast({ message: '账户已保存', type: 'success' });
    } catch {
      showToast({ message: '保存账户失败', type: 'error' });
    }
  };

  const handleDeleteAccount = async (id: string) => {
    try {
      await deleteAccount(id);
      dispatchAccountsChanged();
      showToast({ message: '账户已删除', type: 'success' });
    } catch {
      // error handled in hook
    }
  };

  return (
    <div className="max-w-5xl mx-auto w-full space-y-6">
      <div className="flex items-start justify-between border-b border-slate-800/80 pb-5">
        <div>
          <h2 className="text-2xl font-bold text-slate-100 flex items-center gap-2"><span>账户与密钥</span></h2>
          <p className="text-xs text-slate-400 mt-1">模型账户、凭据和执行身份的归属关系。</p>
        </div>
        <button onClick={() => setShowAddAccountModal(true)} className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold flex items-center gap-2 transition shadow-lg shadow-amber-600/20">
          <i className="fa-solid fa-plus"></i><span>新增账户认证</span>
        </button>
      </div>

      {loading && <div className="text-center text-slate-500 text-xs py-8">加载中...</div>}

      <div className="space-y-3">
        {accounts.map((acc) => (
          <div key={acc.id} className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800 hover:border-slate-700 transition group shadow-sm">
            <div className="flex items-center justify-between">
              <div className="space-y-1 min-w-0">
                <div className="flex items-center space-x-2">
                  <span className="text-xs font-bold text-slate-100 font-mono">{acc.name}</span>
                  {acc.builtin && <span className="px-1.5 py-0.5 rounded-lg bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 text-[9px] font-mono">builtin</span>}
                  {!acc.builtin && <span className="px-1.5 py-0.5 rounded-lg bg-slate-800 text-slate-400 border border-slate-700 text-[9px] font-mono">custom</span>}
                  {acc.hasApiKey && <span className="text-emerald-400 text-[10px]" title="API Key 已设置"><i className="fa-solid fa-key"></i></span>}
                </div>
                <div className="text-[11px] font-mono text-slate-400">{acc.details}</div>
                {acc.models && acc.models.length > 0 && (
                  <div className="flex flex-wrap gap-1 mt-1">
                    {acc.models.slice(0, 5).map((m) => (
                      <span key={m} className="px-1.5 py-0.5 rounded-lg bg-slate-800 border border-slate-700 text-[9px] font-mono text-slate-300">{m}</span>
                    ))}
                    {acc.models.length > 5 && <span className="text-[9px] text-slate-500">+{acc.models.length - 5}</span>}
                  </div>
                )}
              </div>
              <div className="flex items-center space-x-3 shrink-0 pl-3 border-l border-slate-800/80">
                <span className={clsx('px-2.5 py-1 rounded-lg text-[10px] font-mono font-bold uppercase border', acc.type === 'oauth' ? 'bg-amber-500/20 text-amber-300 border-amber-500/40' : 'bg-purple-500/20 text-purple-300 border-purple-500/40')}>
                  {acc.type}
                </span>
                <button onClick={() => setEditingAccount(acc)} className="p-1.5 text-slate-500 hover:text-amber-400 transition" title="编辑账户">
                  <i className="fa-solid fa-pen text-xs"></i>
                </button>
                <button onClick={() => handleDeleteAccount(acc.id)} className="p-1.5 text-slate-500 hover:text-rose-400 transition" title="删除凭据">
                  <i className="fa-regular fa-trash-can text-xs"></i>
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>

      {showAddAccountModal && <AccountEditModal onSave={handleSaveAccount} onClose={() => setShowAddAccountModal(false)} />}
      {editingAccount && <AccountEditModal account={editingAccount} onSave={handleSaveAccount} onClose={() => setEditingAccount(null)} />}
    </div>
  );
}
