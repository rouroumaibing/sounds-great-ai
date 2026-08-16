// Navigation types
export type PrimaryNavType = 'threads' | 'tasks' | 'memory' | 'settings' | 'about' | 'custody' | 'profiles' | 'people';
export type SettingsTabType = 'members' | 'accounts' | 'personas' | 'im' | 'skills' | 'mcp' | 'plugins' | 'market' | 'marketplace' | 'ball' | 'concierge' | 'voice' | 'config' | 'rules' | 'notifications' | 'system' | 'ops' | 'eval' | 'about';
export type DrawerTabType = 'plan' | 'mcp' | 'memory' | 'files' | 'session-chain';
export type ThreadFilterType = 'all' | 'escalated' | 'active';
export type MemberFilterType = 'all' | 'enabled' | 'disabled' | 'oauth' | 'config';

// Dog Agent
export interface DogAgent {
  id: string;
  name: string;
  title: string;
  role: string;
  color: string;
  icon: string;
  adapter: string;
  model: string;
  latency: string;
  statusText: string;
  statusBadgeClass: string;
}

// Tool Log
export interface ToolLog {
  name: string;
  args: string;
  duration: string;
  output: string;
}

// Stream Events
export interface CvoMessageEvent {
  type: 'cvo_message';
  timestamp: string;
  content: string;
}

export interface BreedCardEvent {
  type: 'breed_card';
  breedId: string;
  role: string;
  model: string;
  timestamp: string;
  thinking: string;
  showThinking: boolean;
  content: string;
  tools: ToolLog[];
}

export interface SopGateEvent {
  type: 'sop_gate';
  reason?: string;
}

export interface EscalationOption {
  id: string;
  label: string;
}

export interface CvoEscalationEvent {
  type: 'cvo_escalation';
  escalationTitle?: string;
  options?: EscalationOption[];
}

// WS streaming event types
export interface ThinkingEvent {
  type: 'thinking';
  content: string;
  step: number;
  status: 'running' | 'success' | 'error';
}

export interface ToolCallEvent {
  type: 'tool_call';
  tool: string;
  params: string;
  result: string;
  status: string;
}

export interface CodeDiffEvent {
  type: 'code_diff';
  file: string;
  diff: string;
  action: string;
}

export interface TerminalOutputEvent {
  type: 'terminal_output';
  stream: 'stdout' | 'stderr';
  data: string;
}

export interface ApprovalRequestEvent {
  type: 'approval_request';
  action: string;
  request_id: string;
  impact: string;
}

export interface BreedResponseStartEvent {
  type: 'breed_response_start';
  breed: string;
  timestamp: string;
}

export interface BreedResponseCompleteEvent {
  type: 'breed_response_complete';
  breed: string;
  steps: unknown[];
  // content is populated by history hydration (G9) or by the live terminal
  // BARK_RESULT so assistant answers render as text.
  content?: string;
}

// Live streaming assistant text (G1). Accumulated from AGENT_MESSAGE deltas
// during a run; converted to breed_response_complete on BARK_RESULT.
export interface BreedResponseLiveEvent {
  type: 'breed_response_live';
  breed: string;
  content: string;
}

// Liveness status bar (R8). Surfaces CLI stall (alive-but-silent) / recovery
// so a hanging agent is visible instead of failing silently.
export interface BreedStallWarningEvent {
  type: 'breed_stall_warning';
  breed: string;
  state: string; // "active" | "busy_silent" | "idle_silent" | "dead"
  hard: boolean; // true => hard stall (beyond ProbeStallWarnMs)
  message: string;
}

export interface ErrorEvent {
  type: 'error';
  breed?: string;
  error: string;
  // Optional structured reason code. When present the UI
  // color-codes by tier; otherwise a keyword heuristic is used.
  reason?: string;
  // Structured diagnostics (cliDiagnostics), populated by the
  // backend from ClassifyError/SanitizeStderr. All optional; the UI degrades
  // gracefully when absent.
  summary?: string; // public-safe one-line classification
  hint?: string; // actionable hint
  excerpt?: string; // already server-sanitized (REDACTED-*) stderr excerpt
  source?: string; // where the excerpt came from (gated by allowlist before raw display)
  meta?: Record<string, string>; // redactable context (paths, cli command) for the meta bar
}

export interface Toast {
  id: string;
  message: string;
  type: 'info' | 'warning' | 'error' | 'success';
}

export type StreamEvent =
  | CvoMessageEvent
  | BreedCardEvent
  | SopGateEvent
  | CvoEscalationEvent
  | ThinkingEvent
  | ToolCallEvent
  | CodeDiffEvent
  | TerminalOutputEvent
  | ApprovalRequestEvent
  | BreedResponseStartEvent
  | BreedResponseCompleteEvent
  | BreedResponseLiveEvent
  | BreedStallWarningEvent
  | ErrorEvent;

// Task Plan Step
export interface TaskPlanStep {
  title: string;
  desc: string;
  status: string;
  assignee: string;
  rule: string;
  borderClass: string;
  badgeClass: string;
}

// Thread
export interface Thread {
  id: string;
  title: string;
  created_at: number;
  deleted_at?: number | null;
  events?: StreamEvent[];
  hasEscalation?: boolean;
  agents?: string[];
  updatedAt?: string;
  taskPlanSteps?: TaskPlanStep[];
}

// Settings Member
export interface SettingsMember {
  id: string;
  name: string;
  breed: string;
  color: string;
  icon: string;
  model: string;
  handle: string;
  sessionChain: boolean;
  enabled: boolean;
  provider: string;
  type: string;
  clientId?: string;
  accountRef?: string;
  defaultModel?: string;
  nickname?: string;
  avatar?: string;
  colorPrimary?: string;
  colorSecondary?: string;
  mentionPatterns?: string[];
  personality?: string;
  roleDescription?: string;
  teamStrengths?: string[];
  caution?: string;
  cliCommand?: string;
  outputFormat?: string;
  defaultArgs?: string;
  effort?: string;
  contextWindow?: number;
  maxPromptTokens?: number;
  maxContextTokens?: number;
  maxMessages?: number;
  mcpSupport?: boolean;
  strategy?: string;
  // 派生状态（决策 D2）：绑定账号的密钥/CLI 是否就绪。
  credentialReady?: boolean;
}

// Settings Account
export interface SettingsAccount {
  id: string;
  name: string;
  details: string;
  type: 'oauth' | 'api_key';
  clientId?: string;
  displayName?: string;
  baseUrl?: string;
  models?: string[];
  modelAliases?: Record<string, string>;
  envVars?: Record<string, string>;
  authType?: string;
  mode?: string;
  builtin?: boolean;
  hasApiKey?: boolean;
}

// File Node
export interface FileNode {
  id: string;
  name: string;
  type: 'folder' | 'file';
  path?: string;
  expanded?: boolean;
  children?: FileNode[];
}

// Loaded Skill
export interface LoadedSkill {
  name: string;
  source: string;
}

// MCP Server
export interface McpServer {
  name: string;
  tools: string[];
}

// Shared Memory
export interface SharedMemory {
  id: number;
  type: string;
  time: string;
  fact: string;
  author: string;
}

// Context Menu State
export interface ContextMenuState {
  show: boolean;
  x: number;
  y: number;
  file: FileNode | null;
}

// Settings Nav Menu Item
export interface SettingsNavItem {
  id: SettingsTabType;
  label: string;
  icon: string;
}

// Drawer Tab Item
export interface DrawerTabItem {
  id: DrawerTabType;
  label: string;
  icon: string;
}

// Notification
export interface Notification {
  id: string;
  severity: 'info' | 'warning' | 'error';
  title: string;
  message: string;
  source: string;
  timestamp: string;
  read: boolean;
}

// System Config Group
export interface SystemConfigGroup {
  category: string;
  icon: string;
  items: { key: string; value: string; description?: string }[];
}

// Env Summary (from GET /api/config/env-summary)
export interface EnvVariable {
  key: string;
  value: string;
  sensitive: boolean;
  editable?: boolean;
  description?: string;
}

export interface EnvCategory {
  name: string;
  variables: EnvVariable[];
}

export interface EnvSummary {
  categories: EnvCategory[];
  data_dirs: Record<string, string>;
  storage_mode?: string;
}

// Rules data (from GET /api/rules)
export interface RulesData {
  iron_laws: { id: string; title: string; desc: string }[];
  red_flags: { pattern: string; violation: string; fix: string }[];
  breed_restrictions: { breed: string; can: string; cannot: string }[];
  model_guides: { adapter: string; guide: string }[];
  agents_content: string;
}

// Hook manifest (from GET /api/prompt-injection/manifest)
export interface HookManifestData {
  hooks: {
    id: string;
    name: string;
    stage: string;
    order: number;
    enabled: boolean;
    disableable: boolean;
    resolver: string;
    template: string;
    has_template: boolean;
  }[];
  stages: string[];
}

// Breed Persona (detailed)
export interface BreedPersona {
  id: string;
  name: string;
  englishName: string;
  role: string;
  model: string;
  adapter: string;
  color: string;
  icon: string;
  personality: string;
  capabilities: string[];
  enabled: boolean;
}

// Session Chain
export interface SessionRecord {
  id: string;
  thread_id: string;
  breed_id: string;
  seq: number;
  status: 'active' | 'sealed';
  message_count: number;
  seal_reason?: string;
  created_at: number;
  sealed_at?: number | null;
  cliSessionId?: string;
  dogId?: string;
  compressionCount?: number;
  inputTokens?: number;
  outputTokens?: number;
  contextFillRatio?: number;
}
