import { useCallback, useEffect, useState } from 'react';
import { settingsService } from '../services/settingsService';
import { useAppStore } from '../store/useAppStore';
import type { SettingsMember, SettingsAccount, SystemConfigGroup } from '../types';

export function useSettings() {
  const [members, setMembers] = useState<SettingsMember[]>([]);
  const [accounts, setAccounts] = useState<SettingsAccount[]>([]);
  const [config, setConfig] = useState<SystemConfigGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const showToast = useAppStore((s) => s.showToast);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [m, a, c] = await Promise.all([
        settingsService.getMembers(),
        settingsService.getAccounts(),
        settingsService.getSystemConfig(),
      ]);
      setMembers(m);
      setAccounts(a);
      setConfig(c);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: `加载设置失败: ${msg}`, type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  const addMember = useCallback(async (member: Omit<SettingsMember, 'id'>) => {
    try {
      const newMember = await settingsService.addMember(member);
      setMembers((prev) => [...prev, newMember]);
    } catch (e) {
      showToast({ message: '添加成员失败', type: 'error' });
      throw e;
    }
  }, [showToast]);

  const toggleMemberEnabled = useCallback(async (id: string, enabled: boolean) => {
    try {
      await settingsService.updateMember(id, { enabled });
      setMembers((prev) => prev.map((m) => m.id === id ? { ...m, enabled } : m));
    } catch (e) {
      showToast({ message: '更新成员失败', type: 'error' });
    }
  }, [showToast]);

  const deleteMember = useCallback(async (id: string) => {
    try {
      await settingsService.deleteMember(id);
      setMembers((prev) => prev.filter((m) => m.id !== id));
    } catch (e) {
      showToast({ message: '删除成员失败', type: 'error' });
    }
  }, [showToast]);

  const addAccount = useCallback(async (name: string, provider: string, apiKey: string) => {
    try {
      const newAccount = await settingsService.addAccount(name, provider, apiKey);
      setAccounts((prev) => [...prev, newAccount]);
    } catch (e) {
      showToast({ message: '添加账户失败', type: 'error' });
      throw e;
    }
  }, [showToast]);

  const deleteAccount = useCallback(async (id: string) => {
    try {
      await settingsService.deleteAccount(id);
      setAccounts((prev) => prev.filter((a) => a.id !== id));
    } catch (e) {
      showToast({ message: '删除账户失败', type: 'error' });
    }
  }, [showToast]);

  return {
    members, accounts, config, loading, error,
    addMember, toggleMemberEnabled, deleteMember,
    addAccount, deleteAccount, refetch: fetchAll,
  };
}
