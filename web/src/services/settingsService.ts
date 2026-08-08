import { apiGet, apiPost, apiPatch, apiDelete, ApiError } from './http';
import type { SettingsMemberApi, SettingsAccountApi, SystemConfigApi } from '../types/api';
import type { SettingsMember, SettingsAccount, SystemConfigGroup, EnvSummary, RulesData, HookManifestData } from '../types';

function mapMemberApiToUi(m: SettingsMemberApi): SettingsMember {
  return {
    id: m.id,
    name: m.display_name,
    breed: m.role,
    color: m.color_primary || '#4A90D9',
    icon: 'fa-solid fa-dog',
    model: m.default_model || '',
    handle: `@${m.breed_id}`,
    sessionChain: m.session_chain ?? true,
    enabled: m.enabled,
    provider: m.provider || '',
    type: 'CLI (OAuth)',
    clientId: m.client_id,
    accountRef: m.account_ref,
    defaultModel: m.default_model,
    nickname: m.nickname,
    avatar: m.avatar,
    colorPrimary: m.color_primary,
    colorSecondary: m.color_secondary,
    mentionPatterns: m.mention_patterns,
    personality: m.personality,
    roleDescription: m.role_description,
    teamStrengths: m.team_strengths,
    caution: m.caution,
    cliCommand: m.cli_command,
    outputFormat: m.output_format,
    defaultArgs: m.default_args,
    effort: m.effort,
    contextWindow: m.context_window,
    maxPromptTokens: m.max_prompt_tokens,
    maxContextTokens: m.max_context_tokens,
    maxMessages: m.max_messages,
    mcpSupport: m.mcp_support,
    strategy: m.strategy,
  };
}

function mapAccountApiToUi(a: SettingsAccountApi): SettingsAccount {
  return {
    id: a.id,
    name: a.name || a.provider,
    details: a.key_preview || (a.key_set ? '••••••••' : '未设置'),
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
  async getMembers(): Promise<SettingsMember[]> {
    const data = await apiGet<SettingsMemberApi[]>('/api/settings/members');
    return Array.isArray(data) ? data.map(mapMemberApiToUi) : [];
  },

  async addMember(member: Omit<SettingsMember, 'id'>): Promise<SettingsMember> {
    const data = await apiPost<SettingsMemberApi>('/api/settings/members', {
      breed_id: member.breed,
      display_name: member.name,
      role: member.breed,
      enabled: member.enabled,
      client_id: member.clientId,
      account_ref: member.accountRef,
      default_model: member.defaultModel,
      provider: member.provider,
      nickname: member.nickname,
      avatar: member.avatar,
      color_primary: member.colorPrimary,
      color_secondary: member.colorSecondary,
      mention_patterns: member.mentionPatterns,
      personality: member.personality,
      role_description: member.roleDescription,
      team_strengths: member.teamStrengths,
      caution: member.caution,
      cli_command: member.cliCommand,
      output_format: member.outputFormat,
      default_args: member.defaultArgs,
      effort: member.effort,
      context_window: member.contextWindow,
      max_prompt_tokens: member.maxPromptTokens,
      max_context_tokens: member.maxContextTokens,
      max_messages: member.maxMessages,
      mcp_support: member.mcpSupport,
      session_chain: member.sessionChain,
      strategy: member.strategy,
    });
    return mapMemberApiToUi(data);
  },

  async updateMember(id: string, updates: Partial<SettingsMember>): Promise<void> {
    const body: Record<string, unknown> = {};
    if (updates.name !== undefined) body.display_name = updates.name;
    if (updates.breed !== undefined) body.role = updates.breed;
    if (updates.enabled !== undefined) body.enabled = updates.enabled;
    if (updates.clientId !== undefined) body.client_id = updates.clientId;
    if (updates.accountRef !== undefined) body.account_ref = updates.accountRef;
    if (updates.defaultModel !== undefined) body.default_model = updates.defaultModel;
    if (updates.provider !== undefined) body.provider = updates.provider;
    if (updates.nickname !== undefined) body.nickname = updates.nickname;
    if (updates.avatar !== undefined) body.avatar = updates.avatar;
    if (updates.colorPrimary !== undefined) body.color_primary = updates.colorPrimary;
    if (updates.colorSecondary !== undefined) body.color_secondary = updates.colorSecondary;
    if (updates.mentionPatterns !== undefined) body.mention_patterns = updates.mentionPatterns;
    if (updates.personality !== undefined) body.personality = updates.personality;
    if (updates.roleDescription !== undefined) body.role_description = updates.roleDescription;
    if (updates.teamStrengths !== undefined) body.team_strengths = updates.teamStrengths;
    if (updates.caution !== undefined) body.caution = updates.caution;
    if (updates.cliCommand !== undefined) body.cli_command = updates.cliCommand;
    if (updates.outputFormat !== undefined) body.output_format = updates.outputFormat;
    if (updates.defaultArgs !== undefined) body.default_args = updates.defaultArgs;
    if (updates.effort !== undefined) body.effort = updates.effort;
    if (updates.contextWindow !== undefined) body.context_window = updates.contextWindow;
    if (updates.maxPromptTokens !== undefined) body.max_prompt_tokens = updates.maxPromptTokens;
    if (updates.maxContextTokens !== undefined) body.max_context_tokens = updates.maxContextTokens;
    if (updates.maxMessages !== undefined) body.max_messages = updates.maxMessages;
    if (updates.mcpSupport !== undefined) body.mcp_support = updates.mcpSupport;
    if (updates.sessionChain !== undefined) body.session_chain = updates.sessionChain;
    if (updates.strategy !== undefined) body.strategy = updates.strategy;
    await apiPatch(`/api/settings/members/${id}`, body);
  },

  async deleteMember(id: string): Promise<void> {
    await apiDelete(`/api/settings/members/${id}`);
  },

  async getAccounts(): Promise<SettingsAccount[]> {
    const data = await apiGet<SettingsAccountApi[]>('/api/settings/accounts');
    return Array.isArray(data) ? data.map(mapAccountApiToUi) : [];
  },

  async addAccount(name: string, provider: string, apiKey: string): Promise<SettingsAccount> {
    const data = await apiPost<SettingsAccountApi>('/api/settings/accounts', { name, provider, api_key: apiKey });
    return mapAccountApiToUi(data);
  },

  async addAccountFull(account: Omit<SettingsAccount, 'id'>): Promise<SettingsAccount> {
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
    });
    return mapAccountApiToUi(data);
  },

  async updateAccount(id: string, updates: Partial<SettingsAccount>): Promise<void> {
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
    await apiPatch(`/api/settings/accounts/${id}`, body);
  },

  async deleteAccount(id: string): Promise<void> {
    await apiDelete(`/api/settings/accounts/${id}`);
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
