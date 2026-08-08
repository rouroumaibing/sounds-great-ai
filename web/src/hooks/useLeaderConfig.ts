import { useCallback, useEffect, useState } from 'react';
import { apiGet, apiPatch } from '../services/http';
import { useAppStore } from '../store/useAppStore';

export interface LeaderConfig {
  name: string;
  aliases: string[];
  mentionPatterns: string[];
  timeZone?: string;
  avatar?: string;
  colorPrimary?: string;
  colorSecondary?: string;
}

const DEFAULT_LEADER: LeaderConfig = {
  name: 'You',
  aliases: ['Owner'],
  mentionPatterns: ['@You', '@leader', '@owner'],
  timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone ?? 'Asia/Shanghai',
};

let leaderCache: LeaderConfig | null = null;

export function useLeaderConfig() {
  const [leader, setLeader] = useState<LeaderConfig>(leaderCache ?? DEFAULT_LEADER);
  const [loading, setLoading] = useState(leaderCache === null);
  const showToast = useAppStore((s) => s.showToast);

  const fetchLeader = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiGet<LeaderConfig>('/api/config/leader');
      leaderCache = data;
      setLeader(data);
    } catch {
      // use default on error
      leaderCache = DEFAULT_LEADER;
      setLeader(DEFAULT_LEADER);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (leaderCache) {
      setLeader(leaderCache);
      setLoading(false);
      return;
    }
    fetchLeader();
  }, [fetchLeader]);

  const updateLeader = useCallback(async (cfg: LeaderConfig): Promise<boolean> => {
    try {
      const data = await apiPatch<LeaderConfig>('/api/config/leader', cfg);
      leaderCache = data;
      setLeader(data);
      showToast({ message: 'Leader 设置已保存', type: 'success' });
      return true;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      showToast({ message: `保存 Leader 失败: ${msg}`, type: 'error' });
      return false;
    }
  }, [showToast]);

  return { leader, loading, updateLeader, refetch: fetchLeader };
}
