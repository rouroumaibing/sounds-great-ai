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
  // Final assistant text when available (G9); lets the terminal render without
  // waiting for REST history hydration.
  content?: string;
}

export interface AgentMessagePayload {
  breed: string;
  content: string; // incremental text delta
  done: boolean; // reserved terminal marker
}

// Liveness payload (R8): CLI stall/recovery status surfaced to the client.
export interface LivenessPayload {
  breed: string;
  state: string; // "active" | "busy_silent" | "idle_silent" | "dead"
  hard: boolean; // true => hard stall (beyond ProbeStallWarnMs)
  message: string;
}

export interface BarkErrorPayload {
  breed: string;
  error: string;
  // Structured diagnostics (cliDiagnostics). Optional.
  reason?: string;
  summary?: string;
  hint?: string;
  excerpt?: string;
  source?: string;
  meta?: Record<string, string>;
}

// Carrier health payload (T25 / R6): the backend surfaces a carrier's health
// (quota / structural / transient degradation) so the frontend can render
// upstream model health directly instead of inferring it from raw stream
// events. Keyed in the store by `carrier` (e.g. "claude").
export interface CarrierHealthPayload {
  carrier: string;
  transport?: string; // transport tier that emitted/skipped (e.g. "bg_daemon")
  level: 'online' | 'degraded' | 'offline';
  reason?: string;
  remaining_ms?: number; // ms until the degradation TTL expires
}

export interface SystemNoticePayload {
  severity: 'critical' | 'warning' | 'info' | 'recovery';
  title: string;
  message: string;
  timestamp: string; // ISO 8601
}

// SOP_GATE payload: cross-breed review gate status pushed from the backend.
export interface SopGatePayload {
  reason: string;
  author?: string;
  reviewer?: string;
  blocked?: boolean;
}

// CVO escalation (G4): pushed when the A2A depth hard rail parks the ball
// with the operator; answered via CVO_ESCALATION_RESPONSE. Option labels are
// localized client-side by option id — only semantic ids cross the wire.
export interface CvoEscalationOptionPayload {
  id: string;
  prompt: string;
}

export interface CvoEscalationPayload {
  escalation_id: string;
  reason: string;
  max_depth?: number;
  from_breed?: string;
  to_breed?: string;
  options: CvoEscalationOptionPayload[];
}

// --- CVO_ESCALATION_RESPONSE (client → server) ---
export interface CvoEscalationResponsePayload {
  session_id: string;
  escalation_id: string;
  decision: string; // option id or "intervene"
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

// Typed-lane Shared Memory entry (backend snake_case). Returned by
// /api/memory/lanes/{pending,truth,search} and consumed by the disposition endpoints.
export interface LaneEntryApi {
  id: string;
  type: string; // lane type: taste|profile|entity|person|event|decision|lesson
  content: string;
  source: string;
  timestamp: number; // unix milli
  status: string; // pending|approved|retired|forgotten|deferred
  operator_id?: string; // owner operator; "" = shared
  sensitivity?: string; // data-sensitivity tag (F186); "" = none
  collection_id?: string; // collection/namespace; "" = default
}

// A single memory-recall observation (injection observability, homologous
// clowder recall_events). Content-free metadata only.
export interface RecallEventApi {
  id: string;
  operator_id: string;
  timestamp: number; // unix milli
  kind: string; // push | pull
  trigger: string; // session_bootstrap | cold_context | seal | manual
  entry_ids: string[];
  count: number;
  outcome?: string; // "" | used | ignored (consumption verification, P0-1)
}

// Per-day-window consumption view (homologous clowder RecallLedger funnel +
// CrossCatMetricsComputer unverifiedConsumptionRate + RecallLedgerThreeAxis).
// The three axes (beneficial / unmet / attention) are the semantic quality of
// recall; maturity labels how trustworthy each measurement is.
export interface RecallWindowStatApi {
  total: number;
  used: number;
  ignored: number;
  unverified: number;
  rate: number; // unverified / total, 0..1
  beneficial: number; // used → useful, low attention cost
  unmet: number; // unverified → estimated unmet demand
  attention: number; // ignored → attention cost
  maturity: Record<string, number>; // measured/estimated/lower_bound/none counts
}

// Recall counts within day-windows (homologous clowder RecallLedger funnel).
export type RecallLedgerApi = Record<string, RecallWindowStatApi>; // key "7d"|"14d"|"30d" -> stat

// A single cue-consumption ledger event (Gap4, homologous clowder memCueEvents).
export interface CueEventApi {
  id: string;
  entry_id: string;
  lane: string;
  rank: number;
  score: number;
  consumed: boolean;
  operator_id: string;
  timestamp: number;
}

// An append-only lifecycle-trace record (P1 three-axis / Task #39,
// homologous clowder lifecycle_traces).
export interface LifecycleTraceApi {
  axis: string; // creation|consumption|correction
  entry_id: string;
  lane: string;
  detail: string;
  maturity: string; // measured|estimated|lower_bound|none
  timestamp: number;
}

// LLM abstractive reflection request (P2-6, sanctioned synthesis service).
export interface ReflectRequestApi {
  lane?: string; // optional lane filter
  focus?: string; // optional reflection focus directive
  seed?: boolean; // submit reflection as a pending candidate (human disposition)
  max_chars?: number;
}

// LLM abstractive reflection response (P2-6).
export interface ReflectResponseApi {
  reflection: string;
  count: number; // truth entries reflected over
  seeded_ids?: string[]; // present when seed=true
}

// Typed relationship edge between two memory entries (Gap1, homologous clowder
// edges). 10 relations + edge-level sensitivity/provenance/traversal (clowder
// V18 edge columns).
export interface LaneEdgeApi {
  id: string;
  from_id: string;
  to_id: string;
  relation: string; // one of the 10 LANE_RELATIONS
  edge_sensitivity?: string; // inherit ("") | public|internal|private|restricted
  provenance?: string; // session_seal|manual|import
  traversal_count?: number;
  last_traversed_at?: number;
  operator_id: string;
  timestamp: number;
}

// Normalized signal attached to an entry (Gap1, homologous clowder marker:
// captured/normalized/approved/rejected).
export interface LaneMarkerApi {
  id: string;
  entry_id: string;
  marker_type: string; // e.g. decision|lesson|correction
  content: string;
  status: string; // captured|normalized|approved|rejected
  operator_id: string;
  timestamp: number;
}

// 4-tier sensitivity levels (Gap2, homologous clowder CollectionSensitivity).
export type SensitivityLevel = 'public' | 'internal' | 'private' | 'restricted';

// Relationship kinds offered in the UI — matches the 10 clowder edge relations.
export const LANE_RELATIONS: string[] = [
  'evolved_from',
  'blocked_by',
  'supersedes',
  'invalidates',
  'related',
  'related_to',
  'promoted_from',
  'wikilink',
  'doc_link',
  'feature_ref',
];

// Sensitivity levels offered in the UI (Gap2).
export const SENSITIVITY_LEVELS: SensitivityLevel[] = [
  'public',
  'internal',
  'private',
  'restricted',
];

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

// --- SOP workflow bulletin board (backend snake_case) ---
export type WorkflowCheckStatus = 'attested' | 'verified' | 'unknown';

export interface WorkflowCheck {
  name: string;
  status: WorkflowCheckStatus;
  at: string;
}

export interface WorkflowSopState {
  feature_id: string;
  stage: string;
  baton_holder: string;
  next_skill: string;
  resume_capsule: string;
  checks: WorkflowCheck[];
  updated_at: string;
}
