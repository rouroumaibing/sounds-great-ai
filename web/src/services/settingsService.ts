import { apiGet, apiPost, apiPatch, apiDelete, apiPut, ApiError } from './http';
import type { SettingsAccountApi, SystemConfigApi, RosterEntry, ReviewPolicy, DefaultBreedResponse, BreedOrderResponse } from '../types/api';
import type { SettingsAccount, SystemConfigGroup, EnvSummary, RulesData, HookManifestData } from '../types';
import { useI18n } from '../store/useI18n';

function mapAccountApiToUi(a: SettingsAccountApi): SettingsAccount {
  return {
    id: a.id,
    name: a.name || a.provider,
    details: a.key_preview || (a.key_set ? '••••••••' : useI18n.getState().t('settings.settingsservice.s1')),
    type: (a.auth_type === 'oauth' ? 'oauth' : 'api_key') as 'oauth' | 'api_key',
    clientId: a.client_id,
    displayName: a.display_name,
    baseUrl: a.base_url,
    models: a.models,
    modelAliases: a.model_aliases,
    envVars: a.env_vars,
    authType: a.auth_type,
    mode: a.mode,
    builtin: a.builtin,
    hasApiKey: a.key_set,
  };
}

export const settingsService = {
  // ----- roster (runtime membership meta) -----
  async getRoster(): Promise<Record<string, RosterEntry>> {
    const data = await apiGet<Record<string, RosterEntry>>('/api/settings/roster');
    return data ?? {};
  },

  async updateRosterEntry(id: string, patch: Partial<RosterEntry>): Promise<void> {
    await apiPatch(`/api/settings/roster/${id}`, patch);
  },

  // ----- review policy -----
  async getReviewPolicy(): Promise<ReviewPolicy> {
    const data = await apiGet<ReviewPolicy>('/api/settings/review-policy');
    return data ?? {};
  },

  async updateReviewPolicy(policy: ReviewPolicy): Promise<void> {
    await apiPut('/api/settings/review-policy', policy);
  },

  // ----- default breed -----
  async getDefaultBreed(): Promise<DefaultBreedResponse> {
    const data = await apiGet<DefaultBreedResponse>('/api/config/default-breed');
    return { breed_id: data?.breed_id ?? '', is_override: Boolean(data?.is_override) };
  },

  async setDefaultBreed(breedId: string): Promise<void> {
    await apiPut('/api/config/default-breed', { breed_id: breedId });
  },

  // ----- breed order -----
  async getBreedOrder(): Promise<string[]> {
    const data = await apiGet<BreedOrderResponse>('/api/config/breed-order');
    return data?.order ?? [];
  },

  async setBreedOrder(order: string[]): Promise<void> {
    await apiPut('/api/config/breed-order', { order });
  },

  async getAccounts(): Promise<SettingsAccount[]> {
    const data = await apiGet<SettingsAccountApi[]>('/api/settings/accounts');
    return Array.isArray(data) ? data.map(mapAccountApiToUi) : [];
  },

  async addAccount(name: string, provider: string, apiKey: string): Promise<SettingsAccount> {
    const data = await apiPost<SettingsAccountApi>('/api/settings/accounts', { name, provider, api_key: apiKey });
    return mapAccountApiToUi(data);
  },

  async addAccountFull(account: Omit<SettingsAccount, 'id'>, apiKey?: string): Promise<SettingsAccount> {
    const data = await apiPost<SettingsAccountApi>('/api/settings/accounts', {
      name: account.name,
      provider: account.name,
      client_id: account.clientId,
      display_name: account.displayName,
      base_url: account.baseUrl,
      models: account.models,
      model_aliases: account.modelAliases,
      env_vars: account.envVars,
      auth_type: account.authType,
      mode: account.mode,
      builtin: account.builtin,
      ...(apiKey ? { api_key: apiKey } : {}),
    });
    return mapAccountApiToUi(data);
  },

  async updateAccount(id: string, updates: Partial<SettingsAccount>, apiKey?: string): Promise<void> {
    const body: Record<string, unknown> = {};
    if (updates.name !== undefined) body.name = updates.name;
    if (updates.clientId !== undefined) body.client_id = updates.clientId;
    if (updates.displayName !== undefined) body.display_name = updates.displayName;
    if (updates.baseUrl !== undefined) body.base_url = updates.baseUrl;
    if (updates.models !== undefined) body.models = updates.models;
    if (updates.modelAliases !== undefined) body.model_aliases = updates.modelAliases;
    if (updates.envVars !== undefined) body.env_vars = updates.envVars;
    if (updates.authType !== undefined) body.auth_type = updates.authType;
    if (updates.mode !== undefined) body.mode = updates.mode;
    if (updates.builtin !== undefined) body.builtin = updates.builtin;
    if (apiKey !== undefined && apiKey !== '') body.api_key = apiKey;
    await apiPatch(`/api/settings/accounts/${id}`, body);
  },

  async deleteAccount(id: string, opts?: { force?: boolean }): Promise<void> {
    const qs = opts?.force ? '?force=true' : '';
    await apiDelete(`/api/settings/accounts/${id}${qs}`);
  },

  async getSystemConfig(): Promise<SystemConfigGroup[]> {
    const data = await apiGet<SystemConfigApi[]>('/api/settings/config');
    if (!Array.isArray(data)) return [];
    const groups: Record<string, SystemConfigGroup> = {};
    for (const item of data) {
      if (!groups[item.category]) {
        groups[item.category] = { category: item.category, icon: 'fa-solid fa-sliders', items: [] };
      }
      groups[item.category].items.push({ key: item.key, value: item.value });
    }
    return Object.values(groups);
  },

  async getEnvSummary(): Promise<EnvSummary> {
    try {
      return await apiGet<EnvSummary>('/api/config/env-summary');
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.status === 501)) {
        return { categories: [], data_dirs: {} };
      }
      throw e;
    }
  },

  async updateEnv(updates: { key: string; value: string }[]): Promise<string[]> {
    const data = await apiPatch<{ updated: string[] }>('/api/config/env', { updates });
    return data.updated ?? [];
  },

  async getRules(): Promise<RulesData> {
    return await apiGet<RulesData>('/api/rules');
  },

  async getHookManifest(): Promise<HookManifestData> {
    return await apiGet<HookManifestData>('/api/prompt-injection/manifest');
  },

  async getCompilePreview(breed: string): Promise<string> {
    try {
      const data = await apiGet<{ compiled: string }>(`/api/prompt-injection/preview?breed=${encodeURIComponent(breed)}`);
      return data.compiled ?? '';
    } catch {
      return '';
    }
  },
};
