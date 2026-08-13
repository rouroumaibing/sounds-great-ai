// WebSocket envelope (all messages, both directions)
export interface WsEvent {
  type: string;
  session_id: string;
  timestamp: number;
  seq?: number; // sequence number for gap detection
  payload: unknown;
}

// --- BreedConfig (new variant-based format) ---
export interface BreedColor {
  primary: string;
  secondary: string;
}

export interface CLIConfig {
  command: string;
  output_format: string;
  default_args?: string[];
  effort?: string;
}

export interface ContextBudget {
  max_prompt_tokens?: number;
  max_context_tokens?: number;
  max_messages?: number;
  max_content_length_per_msg?: number;
}

export interface VoiceConfig {
  voice?: string;
  lang_code?: string;
  speed?: number;
  ref_audio?: string;
  ref_text?: string;
  instruct?: string;
  temperature?: number;
}

export interface Variant {
  id: string;
  client_id: string;
  default_model: string;
  mcp_support: boolean;
  cli: CLIConfig;
  system_prompt?: string;
  personality?: string;
  strengths?: string[];
  team_strengths?: string;
  caution?: string;
  context_budget?: ContextBudget;
  variant_label?: string;
  voice_config?: VoiceConfig;
  account_ref?: string;
  provider?: string;
  session_chain?: boolean;
  strategy?: string;
  auto_compact_token_limit?: number;
}

export interface BreedConfig {
  id: string;
  name: string;
  display_name: string;
  avatar: string;
  color?: BreedColor;
  personality: string;
  role_description?: string;
  team_strengths?: string;
  mention_patterns: string[];
  roles: string[];
  default_variant_id: string;
  variants: Variant[];
  source: 'system' | 'user' | 'plugin';
  enabled: boolean;
  nickname?: string;
  caution?: string;
}

// Roster entry (runtime membership meta). Keyed by breed id in the roster map.
export interface RosterEntry {
  family: string;
  roles?: string[];
  lead?: boolean;
  available?: boolean;
  evaluation?: string;
  // 派生状态（决策 D2）：成员绑定账号的密钥/CLI 是否就绪。
  // 仅当 available && credential_ready 时成员真正可用（就绪态）。
  credential_ready?: boolean;
}

// Pack-level review policy.
export interface ReviewPolicy {
  require_different_breed?: boolean;
  prefer_active_in_thread?: boolean;
  exclude_unavailable?: boolean;
  preferred_roles?: string[];
}

// Default-breed / breed-order config payloads.
export interface DefaultBreedResponse {
  breed_id: string;
  is_override: boolean;
}

export interface BreedOrderResponse {
  order: string[];
}

// --- TaskInput (Go struct WITHOUT json tags → PascalCase) ---
export interface ExecutionContext {
  user_id: string;
  session_id: string;
  workspace: string;
  trace_id: string;
  permissions: unknown;
  metadata: unknown;
}

export interface TaskInput {
  Query: string;
  Command: string;
  Path: string;
  Context: ExecutionContext;
  Previous: unknown;
  CapabilityConfig: unknown;
}

// --- TaskOutput (Go struct WITHOUT json tags → PascalCase) ---
export interface TaskOutput {
  Success: boolean;
  Error: string;
  Steps: unknown[];
  Result: unknown;
}

// --- WebSocket payload types (server → client) ---
export interface BarkStartPayload {
  breed: string;
  session_id: string;
  query: string;
}

export interface ThinkingPayload {
  step: number;
  content: string;
  timestamp: number;
}

export interface ToolCallPayload {
  tool: string;
  params: string;
  result: string;
  status: string;
}

export interface CodeDiffPayload {
  file: string;
  diff: string;
  action: string;
}

export interface TerminalOutputPayload {
  stream: 'stdout' | 'stderr';
  data: string;
}

export interface HitlApprovalPayload {
  action: string;
  approved: boolean;
  request_id: string;
  impact: string;
}

export interface BarkResultPayload {
  breed: string;
  success: boolean;
  steps: unknown[];
}

export interface BarkErrorPayload {
  breed: string;
  error: string;
}

export interface SystemNoticePayload {
  severity: 'critical' | 'warning' | 'info' | 'recovery';
  title: string;
  message: string;
  timestamp: string; // ISO 8601
}

export interface BarkRejectedPayload {
  reason: string;
  max: number;
}

export interface ErrorPayload {
  error: string;
}

// --- HITL_RESPONSE (client → server) ---
export interface HitlResponsePayload {
  request_id: string;
  approved: boolean;
  reason: string;
}

// --- RAG types ---
export interface Retiree {
  id: string;
  retired_at: number;
  reason: string;
}

export interface RagBackendInfo {
  active: string;
  retirees: Retiree[];
}

export interface SyncProgress {
  from: string;
  to: string;
  current: number;
  total: number;
  status: 'running' | 'completed' | 'error';
  error?: string;
}

// --- Settings types (backend snake_case) ---
export interface SettingsMemberApi {
  id: string;
  breed_id: string;
  display_name: string;
  role: string;
  enabled: boolean;
  created_at: number;
  client_id?: string;
  account_ref?: string;
  default_model?: string;
  provider?: string;
  nickname?: string;
  avatar?: string;
  color_primary?: string;
  color_secondary?: string;
  mention_patterns?: string[];
  personality?: string;
  role_description?: string;
  team_strengths?: string[];
  caution?: string;
  cli_command?: string;
  output_format?: string;
  default_args?: string;
  effort?: string;
  context_window?: number;
  max_prompt_tokens?: number;
  max_context_tokens?: number;
  max_messages?: number;
  mcp_support?: boolean;
  session_chain?: boolean;
  strategy?: string;
}

export interface SettingsAccountApi {
  id: string;
  provider: string;
  key_preview: string;
  key_set: boolean;
  updated_at: number;
  name?: string;
  client_id?: string;
  display_name?: string;
  base_url?: string;
  models?: string[];
  model_aliases?: Record<string, string>;
  env_vars?: Record<string, string>;
  auth_type?: string;
  mode?: string;
  builtin?: boolean;
}

export interface SystemConfigApi {
  key: string;
  value: string;
  category: string;
}

// --- Session chain types (backend snake_case) ---
export interface SessionRecordApi {
  id: string;
  thread_id: string;
  breed_id: string;
  seq: number;
  status: 'active' | 'sealed';
  message_count: number;
  seal_reason: string;
  created_at: number;
  sealed_at: number | null;
}

// --- Memory types (backend snake_case) ---
export interface MemoryEvidenceApi {
  id: string;
  thread_id: string;
  type: 'evidence' | 'decision' | 'lesson';
  title: string;
  content: string;
  tags: string[];
  created_at: number;
}

// --- Thread types (backend snake_case) ---
export interface ThreadApi {
  id: string;
  title: string;
  created_at: number;
  deleted_at: number | null;
}

// --- RAG types (backend snake_case) ---
export interface RagRetireeApi {
  id: string;
  retired_at: number;
  reason: string;
}

export interface RagBackendApi {
  active: string;
  retirees: RagRetireeApi[];
}

export interface SyncProgressApi {
  from: string;
  to: string;
  current: number;
  total: number;
  status: 'running' | 'completed' | 'error';
  error?: string;
}
