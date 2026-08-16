// Frontend mirror of CliDiagnosticsPanel reasonCode tiers.
// The backend classifies failures into 4 tiers (see cli-diagnostics.ts):
//   T1 user-must-fix (red), T2 transient-retry (amber), T3 system/env (slate),
//   T4 cognitive/context (purple). When the backend supplies `reason` we map
//   directly; otherwise we fall back to a keyword heuristic on the message.

export type DiagnosticTier = 'error' | 'warning' | 'info' | 'cognitive';

interface TierStyle {
  // Tailwind class fragments (dark theme).
  border: string;
  bg: string;
  text: string;
  icon: string;
}

export const TIER_STYLES: Record<DiagnosticTier, TierStyle> = {
  error: {
    border: 'border-rose-500/40',
    bg: 'bg-rose-500/5',
    text: 'text-rose-300',
    icon: 'fa-circle-exclamation',
  },
  warning: {
    border: 'border-amber-500/40',
    bg: 'bg-amber-500/5',
    text: 'text-amber-300',
    icon: 'fa-triangle-exclamation',
  },
  info: {
    border: 'border-slate-500/40',
    bg: 'bg-slate-500/5',
    text: 'text-slate-300',
    icon: 'fa-server',
  },
  cognitive: {
    border: 'border-purple-500/40',
    bg: 'bg-purple-500/5',
    text: 'text-purple-300',
    icon: 'fa-brain',
  },
};

// Structured reason codes → tier (subset of the 18 codes; extend as the
// backend begins emitting `reason`).
const REASON_TIER: Record<string, DiagnosticTier> = {
  auth_failed: 'error',
  invalid_config: 'error',
  model_not_found: 'error',
  quota_exceeded: 'error',
  context_window_exceeded: 'cognitive',
  invalid_thinking_signature: 'cognitive',
  tool_call_parse_failed: 'cognitive',
  upstream_policy_reject: 'cognitive',
  network_error: 'warning',
  server_overloaded: 'warning',
  cli_response_timeout: 'warning',
  cli_stall_timeout: 'warning',
  spawn_failed: 'info',
  session_not_found: 'info',
  silent_completion: 'info',
};

const KEYWORD_TIER: Array<[RegExp, DiagnosticTier]> = [
  [/(auth|api[_ ]?key|401|403|quota|rate[_ ]?limit|额度|认证|密钥|permission)/i, 'error'],
  [/(context|token|上下文|thinking|提示词)/i, 'cognitive'],
  [/(timeout|timed[_ ]?out|network|429|overload|连接|econn|stall|无响应|超时)/i, 'warning'],
];

export function classifyErrorTier(reason?: string, message = ''): DiagnosticTier {
  if (reason && REASON_TIER[reason]) return REASON_TIER[reason];
  for (const [re, tier] of KEYWORD_TIER) {
    if (re.test(message)) return tier;
  }
  return 'info';
}

// KNOWN_EXCERPT_SOURCES mirrors the allowlist: only excerpts from a
// trusted, server-sanitized source may be shown verbatim. Anything else is
// withheld so an untrusted upstream cannot leak raw stderr through the UI.
export const KNOWN_EXCERPT_SOURCES = new Set<string>([
  'cli_stderr',
  'cli_stream',
  'server',
]);

// PATH_RE matches absolute paths under common roots so they can be redacted.
const PATH_RE = /\/(?:Users|home|root|tmp|var|opt|srv)\/[^\s"'`<]+/g;

// sanitizePathLeaks redacts absolute filesystem paths to a home-relative `~`
// or a generic `[path]` token, mirroring the sanitizePathLeaks meta bar.
// It never reveals the user's real home directory or username.
export function sanitizePathLeaks(input: string): string {
  if (!input) return input;
  let out = input;
  const home =
    (typeof process !== 'undefined' && process.env && process.env.HOME) || '';
  if (home && out.startsWith(home)) {
    out = '~' + out.slice(home.length);
  }
  out = out.replace(PATH_RE, (m) =>
    m.startsWith('/Users/') || m.startsWith('/home/') || m.startsWith('/root/')
      ? '[path]'
      : '[path]'
  );
  return out;
}

// safeExcerpt returns the raw excerpt only when its source is allowlisted;
// otherwise it returns a safe placeholder so the UI never leaks untrusted raw
// stderr. The backend gates the collapsible "details" the same way.
export function safeExcerpt(
  excerpt?: string,
  source?: string
): { show: boolean; text: string } {
  if (!excerpt) return { show: false, text: '' };
  if (source && KNOWN_EXCERPT_SOURCES.has(source)) {
    return { show: true, text: sanitizePathLeaks(excerpt) };
  }
  return {
    show: false,
    text: '原始错误详情已隐藏（来源未授权展示）。',
  };
}

// redactMeta returns a copy of meta with any path-bearing values redacted,
// for safe display in the meta bar.
export function redactMeta(meta?: Record<string, string>): Record<string, string> {
  if (!meta) return {};
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(meta)) {
    out[k] = sanitizePathLeaks(v);
  }
  return out;
}
