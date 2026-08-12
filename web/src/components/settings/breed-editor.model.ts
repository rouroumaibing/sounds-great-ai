// 添加成员弹窗的数据模型层（表单状态 + 构造 BreedConfig / Variant），
// 适配到 Sounds Great AI 的 BreedConfig / Variant 数据层。
//
// 设计要点（按 SG 实际后端能力裁剪）：
// - 表单状态映射：扁平表单状态由 initialState(breed?) 从 BreedConfig 反序列化、
//   buildBreedPayload 序列化回 BreedConfig。
// - 剥离 SG 后端不存在的字段：codex / acp / transport / antigravity / strategy 独立端点。
// - teamStrengths 与 strengths 在 SG 合并为单一字段（BreedConfig.team_strengths）。
// - CLI 配置（command / output_format / effort / default_args / auto_compact）保留在 Advanced 分区。

import type { BreedConfig, VoiceConfig } from '../../types/api';
import type { SettingsAccount } from '../../types';
import { CLIENT_IDS, providerFromClientId } from '../../constants/clientIds';

export type ClientId = string;
export type SessionChainValue = 'true' | 'false';
export type StrategyValue = 'handoff' | 'compress' | 'hybrid' | '';

export interface BreedEditorFormState {
  dogId: string;
  name: string;
  displayName: string;
  variantLabel: string;
  nickname: string;
  avatar: string;
  colorPrimary: string;
  colorSecondary: string;
  mentionPatterns: string;
  roleDescription: string;
  personality: string;
  teamStrengths: string;
  caution: string;
  clientId: string;
  accountRef: string;
  defaultModel: string;
  commandArgs: string;
  outputFormat: string;
  cliConfigArgs: string[];
  cliEffort: string;
  autoCompact: string;
  mcpSupport: boolean;
  sessionChain: SessionChainValue;
  strategy: StrategyValue;
  enabled: boolean;
  maxPromptTokens: string;
  maxContextTokens: string;
  maxMessages: string;
  maxContentLengthPerMsg: string;
  voiceVoice: string;
  voiceLangCode: string;
  voiceSpeed: string;
  voiceRefAudio: string;
  voiceRefText: string;
  voiceInstruct: string;
  voiceTemperature: string;
}

export const CLIENT_OPTIONS: Array<{ value: string; label: string }> = CLIENT_IDS.map((c) => ({
  value: c.id,
  label: c.label,
}));

export const SESSION_CHAIN_OPTIONS: Array<{ value: SessionChainValue; label: string }> = [
  { value: 'true', label: 'true' },
  { value: 'false', label: 'false' },
];

export const SESSION_STRATEGY_OPTIONS: Array<{ value: StrategyValue; label: string }> = [
  { value: '', label: '—' },
  { value: 'handoff', label: 'handoff' },
  { value: 'compress', label: 'compress' },
  { value: 'hybrid', label: 'hybrid' },
];

export const OUTPUT_FORMAT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: '—' },
  { value: 'text', label: 'text' },
  { value: 'json', label: 'json' },
  { value: 'stream', label: 'stream' },
];

export const CLI_EFFORT_OPTIONS: Array<{ value: string; label: string }> = [
  { value: '', label: '—' },
  { value: 'low', label: 'low' },
  { value: 'medium', label: 'medium' },
  { value: 'high', label: 'high' },
];

export function splitMentionPatterns(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

export function normalizeMentionPattern(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '';
  return trimmed.startsWith('@') ? trimmed : `@${trimmed}`;
}

export function deriveModelMentionPattern(model: string): string {
  const modelId = model.trim().split('/').filter(Boolean).at(-1)?.trim();
  if (!modelId) return '';
  const alias = modelId
    .replace(/^[@.]+/, '')
    .replace(/\s+/g, '-')
    .replace(/[^A-Za-z0-9_.-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[._-]+|[._-]+$/g, '');
  return alias ? normalizeMentionPattern(alias) : '';
}

export function withDefaultModelMentionPattern(form: BreedEditorFormState): BreedEditorFormState {
  const modelAlias = deriveModelMentionPattern(form.defaultModel);
  if (!modelAlias) return form;
  const aliases = splitMentionPatterns(form.mentionPatterns).map(normalizeMentionPattern).filter(Boolean);
  if (aliases.length > 0) return form;
  return { ...form, mentionPatterns: joinTags([modelAlias]) };
}

export function joinTags(tags: string[]): string {
  return tags.join(', ');
}

export function splitStrengthTags(raw: string): string[] {
  return raw
    .split(/[\n,]+/)
    .map((value) => value.trim())
    .filter(Boolean);
}

export function autoSlug(name: string, currentId?: string): string {
  const slug = name
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/^[^a-z]+/, '')
    .replace(/-+$/, '')
    .replace(/-{2,}/g, '-')
    .slice(0, 40);
  if (/^[a-z]/.test(slug)) return slug;
  if (currentId && /^dog-[a-z0-9]+$/.test(currentId)) return currentId;
  const rand = Math.random().toString(36).substring(2, 10);
  return `dog-${rand}`;
}

/**
 * SG 账户过滤：SettingsAccount 已携带 clientId，直接按 clientId 匹配；
 * 未选择 client 时返回全部（与现有 SG 行为一致）。
 *
 * D5：api_key 为通用维度，不设厂商白名单。clientId 为空（未配置）的账号视为
 * 通用 api_key 账号，可绑定任意 clientId 成员，过滤时一并包含。
 */
export function filterAccounts(clientId: string, profiles: SettingsAccount[]): SettingsAccount[] {
  if (!clientId) return profiles;
  return profiles.filter((profile) => profile.clientId === clientId || !profile.clientId);
}

export function initialState(breed?: BreedConfig | null): BreedEditorFormState {
  const variant = breed?.variants?.[0];
  const cli = variant?.cli;
  const voice = variant?.voice_config;
  const budget = variant?.context_budget;
  const name = breed?.name ?? '';
  const displayName = breed?.display_name ?? breed?.name ?? '';
  return {
    dogId: breed?.id ?? '',
    name,
    displayName,
    variantLabel: variant?.variant_label ?? '',
    nickname: breed?.nickname ?? '',
    avatar: breed?.avatar ?? '',
    colorPrimary: breed?.color?.primary ?? '',
    colorSecondary: breed?.color?.secondary ?? '',
    mentionPatterns: joinTags(breed?.mention_patterns ?? []),
    roleDescription: breed?.role_description ?? '',
    personality: breed?.personality ?? '',
    teamStrengths: breed?.team_strengths ?? '',
    caution: breed?.caution ?? '',
    clientId: variant?.client_id ?? 'claude',
    accountRef: variant?.account_ref ?? '',
    defaultModel: variant?.default_model ?? '',
    commandArgs: cli?.command ?? '',
    outputFormat: cli?.output_format ?? '',
    cliConfigArgs: [...(cli?.default_args ?? [])],
    cliEffort: cli?.effort ?? '',
    autoCompact: variant?.auto_compact_token_limit != null ? String(variant.auto_compact_token_limit) : '',
    mcpSupport: variant?.mcp_support ?? false,
    sessionChain: (variant?.session_chain ?? true) ? 'true' : 'false',
    strategy: (variant?.strategy as StrategyValue) ?? '',
    enabled: breed?.enabled ?? true,
    maxPromptTokens: budget?.max_prompt_tokens != null ? String(budget.max_prompt_tokens) : '',
    maxContextTokens: budget?.max_context_tokens != null ? String(budget.max_context_tokens) : '',
    maxMessages: budget?.max_messages != null ? String(budget.max_messages) : '',
    maxContentLengthPerMsg:
      budget?.max_content_length_per_msg != null ? String(budget.max_content_length_per_msg) : '',
    voiceVoice: voice?.voice ?? '',
    voiceLangCode: voice?.lang_code ?? '',
    voiceSpeed: voice?.speed != null ? String(voice.speed) : '',
    voiceRefAudio: voice?.ref_audio ?? '',
    voiceRefText: voice?.ref_text ?? '',
    voiceInstruct: voice?.instruct ?? '',
    voiceTemperature: voice?.temperature != null ? String(voice.temperature) : '',
  };
}

function trimText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function optionalPositiveInteger(raw: string, fieldName: string): number | undefined {
  const trimmed = raw.trim();
  if (!trimmed) return undefined;
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isFinite(parsed) || parsed <= 0 || String(parsed) !== trimmed) {
    throw new Error(`${fieldName} 必须是正整数`);
  }
  return parsed;
}

export function buildVoiceConfig(form: BreedEditorFormState): VoiceConfig | undefined {
  const voice = trimText(form.voiceVoice);
  const langCode = trimText(form.voiceLangCode);
  if (!voice) return undefined;
  if (!langCode) return undefined;
  const speed = Number.parseFloat(form.voiceSpeed);
  const temperature = Number.parseFloat(form.voiceTemperature);
  return {
    voice,
    lang_code: langCode,
    ...(Number.isFinite(speed) && speed > 0 ? { speed } : {}),
    ...(trimText(form.voiceRefAudio) ? { ref_audio: trimText(form.voiceRefAudio) } : {}),
    ...(trimText(form.voiceRefText) ? { ref_text: trimText(form.voiceRefText) } : {}),
    ...(trimText(form.voiceInstruct) ? { instruct: trimText(form.voiceInstruct) } : {}),
    ...(Number.isFinite(temperature) && temperature >= 0 ? { temperature } : {}),
  };
}

export function buildContextBudget(form: BreedEditorFormState) {
  const values = [form.maxPromptTokens, form.maxContextTokens, form.maxMessages, form.maxContentLengthPerMsg].map(
    (value) => value.trim(),
  );
  const filledCount = values.filter((value) => value.length > 0).length;
  if (filledCount === 0) return undefined;
  if (filledCount !== values.length) {
    throw new Error('上下文预算要么全部留空，要么 4 项都填写');
  }
  const parsed = values.map((value) => Number.parseInt(value, 10));
  if (parsed.some((value) => !Number.isFinite(value) || value <= 0)) {
    throw new Error('上下文预算必须是正整数');
  }
  return {
    max_prompt_tokens: parsed[0]!,
    max_context_tokens: parsed[1]!,
    max_messages: parsed[2]!,
    max_content_length_per_msg: parsed[3]!,
  };
}

/**
 * 由扁平表单状态构造 BreedConfig。
 * breed 参数用于保留 id / source / enabled 等编辑态元信息。
 */
export function buildBreedPayload(form: BreedEditorFormState, breed?: BreedConfig | null): BreedConfig {
  const contextBudget = buildContextBudget(form);
  const name = trimText(form.name);
  const displayName = trimText(form.displayName) || name;
  const createName = name || displayName;
  const updateName = name || displayName || breed?.name || breed?.display_name || '';
  const trimmedAccountRef = trimText(form.accountRef);
  const voiceConfig = buildVoiceConfig(form);
  const autoCompact = optionalPositiveInteger(form.autoCompact, '自动压缩阈值');

  const common = {
    id: breed?.id ?? '',
    name: breed ? updateName : createName,
    display_name: displayName,
    avatar: trimText(form.avatar),
    color: {
      primary: trimText(form.colorPrimary),
      secondary: trimText(form.colorSecondary),
    },
    personality: trimText(form.personality),
    role_description: trimText(form.roleDescription) || undefined,
    team_strengths: trimText(form.teamStrengths),
    caution: trimText(form.caution) || undefined,
    nickname: trimText(form.nickname) || undefined,
    mention_patterns: Array.from(
      new Set(splitMentionPatterns(form.mentionPatterns).map(normalizeMentionPattern).filter(Boolean)),
    ),
    roles: breed?.roles ?? [],
    default_variant_id: 'default',
    variants: [
      {
        id: 'default',
        client_id: form.clientId,
        provider: providerFromClientId(form.clientId),
        account_ref: trimmedAccountRef || undefined,
        default_model: trimText(form.defaultModel),
        mcp_support: form.mcpSupport,
        session_chain: form.sessionChain === 'true',
        strategy: (form.strategy || undefined) as BreedConfig['variants'][number]['strategy'],
        variant_label: trimText(form.variantLabel) || undefined,
        voice_config: voiceConfig,
        cli: {
          command: trimText(form.commandArgs),
          output_format: trimText(form.outputFormat),
          default_args: (form.cliConfigArgs ?? []).filter((arg) => arg.trim().length > 0),
          effort: trimText(form.cliEffort) || undefined,
        },
        ...(contextBudget ? { context_budget: contextBudget } : {}),
        ...(autoCompact != null ? { auto_compact_token_limit: autoCompact } : {}),
      },
    ],
    source: breed?.source ?? 'user',
    enabled: form.enabled,
  };

  return common;
}
