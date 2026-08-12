import { useCallback, useEffect, useState } from 'react';
import { settingsService } from '../services/settingsService';
import { useAppStore } from '../store/useAppStore';
import type { SettingsAccount, SystemConfigGroup } from '../types';
import type { RosterEntry as RosterEntryType } from '../types/api';
import { useI18n } from '../store/useI18n';

export function useSettings() {
  const [roster, setRoster] = useState<Record<string, RosterEntryType>>({});
  const [accounts, setAccounts] = useState<SettingsAccount[]>([]);
  const [config, setConfig] = useState<SystemConfigGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { t } = useI18n();
  const showToast = useAppStore((s) => s.showToast);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [r, a, c] = await Promise.all([
        settingsService.getRoster(),
        settingsService.getAccounts(),
        settingsService.getSystemConfig(),
      ]);
      setRoster(r);
      setAccounts(a);
      setConfig(c);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
      showToast({ message: t('hooks.usesettings.s1').replace('{msg}', msg), type: 'error' });
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { fetchAll(); }, [fetchAll]);

  const updateRosterEntry = useCallback(async (id: string, patch: Partial<RosterEntryType>) => {
    try {
      await settingsService.updateRosterEntry(id, patch);
      setRoster((prev) => ({ ...prev, [id]: { ...(prev[id] ?? {}), ...patch } as RosterEntryType }));
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: t('hooks.usesettings.s3').replace('{msg}', msg), type: 'error' });
      throw e;
    }
  }, [showToast]);

  const addAccount = useCallback(async (name: string, provider: string, apiKey: string) => {
    try {
      const newAccount = await settingsService.addAccount(name, provider, apiKey);
      setAccounts((prev) => [...prev, newAccount]);
    } catch (e) {
      showToast({ message: t('hooks.usesettings.s5'), type: 'error' });
      throw e;
    }
  }, [showToast]);

  const deleteAccount = useCallback(async (id: string, force = false) => {
    try {
      await settingsService.deleteAccount(id, force ? { force: true } : undefined);
      setAccounts((prev) => prev.filter((a) => a.id !== id));
    } catch (e) {
      // Re-throw so callers can handle specific cases (e.g. 409 conflict).
      throw e;
    }
  }, []);

  return {
    roster, accounts, config, loading, error,
    updateRosterEntry,
    addAccount, deleteAccount, refetch: fetchAll,
  };
}
