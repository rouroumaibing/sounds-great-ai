package unified

import (
	"regexp"
)

type ErrorReasonCode string

const (
	ReasonAuthFailed            ErrorReasonCode = "auth_failed"
	ReasonQuotaExceeded         ErrorReasonCode = "quota_exceeded"
	ReasonNetworkError          ErrorReasonCode = "network_error"
	ReasonModelNotFound         ErrorReasonCode = "model_not_found"
	ReasonSpawnFailed           ErrorReasonCode = "spawn_failed"
	ReasonContextWindowExceeded ErrorReasonCode = "context_window_exceeded"
	ReasonServerOverloaded      ErrorReasonCode = "server_overloaded"
	ReasonInvalidConfig         ErrorReasonCode = "invalid_config"
	ReasonSessionNotFound       ErrorReasonCode = "session_not_found"
	ReasonToolCallParseFailed   ErrorReasonCode = "tool_call_parse_failed"
	ReasonSilentCompletion      ErrorReasonCode = "silent_completion"
	ReasonMissingRollout        ErrorReasonCode = "missing_rollout"
	ReasonInvalidThinkingSig    ErrorReasonCode = "invalid_thinking_signature"
	// R7: align reasonCode vocabulary. stall_timeout is driven by
	// the liveness probe (set on the SpawnHandle); response_timeout and
	// policy_reject are classifier-derived from stderr/stream error text.
	ReasonStallTimeout         ErrorReasonCode = "cli_stall_timeout"
	ReasonResponseTimeout      ErrorReasonCode = "cli_response_timeout"
	ReasonUpstreamPolicyReject ErrorReasonCode = "upstream_policy_reject"
)

type CliDiagnostics struct {
	ReasonCode    ErrorReasonCode
	PublicSummary string
	PublicHint    string
	SafeExcerpt   string
	ExitCode      *int
	Signal        string
}

var classifierPatterns = []struct {
	code  ErrorReasonCode
	regex *regexp.Regexp
}{
	{ReasonAuthFailed, regexp.MustCompile(`(?i)(invalid\s+api\s+key|authentication\s+failed|unauthorized|401)`)},
	{ReasonQuotaExceeded, regexp.MustCompile(`(?i)(quota|rate\s+limit|429|too\s+many\s+requests)`)},
	{ReasonNetworkError, regexp.MustCompile(`(?i)(ECONNREFUSED|ENOTFOUND|ETIMEDOUT|network\s+error|connection\s+refused)`)},
	{ReasonModelNotFound, regexp.MustCompile(`(?i)(model\s+not\s+found|unknown\s+model|invalid\s+model)`)},
	{ReasonContextWindowExceeded, regexp.MustCompile(`(?i)(context\s+window|token\s+limit|maximum\s+context)`)},
	{ReasonServerOverloaded, regexp.MustCompile(`(?i)(overloaded|503|service\s+unavailable)`)},
	{ReasonInvalidConfig, regexp.MustCompile(`(?i)(invalid\s+config|configuration\s+error|missing\s+required)`)},
	{ReasonSessionNotFound, regexp.MustCompile(`(?i)(session\s+not\s+found|no\s+such\s+session)`)},
	{ReasonToolCallParseFailed, regexp.MustCompile(`(?i)(tool\s+call.*parse|invalid\s+tool\s+call)`)},
	{ReasonInvalidThinkingSig, regexp.MustCompile(`(?i)(invalid.*thinking.*signature)`)},
	{ReasonMissingRollout, regexp.MustCompile(`(?i)(missing\s+rollout|rollout\s+not\s+found)`)},
	{ReasonResponseTimeout, regexp.MustCompile(`(?i)(timeout|timed out|deadline exceeded|504|408|request timeout)`)},
	{ReasonUpstreamPolicyReject, regexp.MustCompile(`(?i)(policy|reject|not allowed|forbidden|403|upstream.*reject)`)},
}

func ClassifyError(stderr string) ErrorReasonCode {
	for _, p := range classifierPatterns {
		if p.regex.MatchString(stderr) {
			return p.code
		}
	}
	return ""
}

var (
	ansiRegex        = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	jwtRegex         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	pemRegex         = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[\s\S]*?-----END [A-Z ]+-----`)
	cookieRegex      = regexp.MustCompile(`(?i)(cookie|session[_-]?id)\s*[=:]\s*[^\s;]+`)
	urlQueryRegex    = regexp.MustCompile(`[?&][^=]+=[^&\s]+`)
	tokenKVRegex     = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[=:]\s*[^\s]+`)
	pathRegex        = regexp.MustCompile(`/(?:Users|home|root|tmp|var|opt|srv)/[^\s]+`)
	highEntropyRegex = regexp.MustCompile(`[A-Za-z0-9]{32,}`)
)

func SanitizeStderr(stderr string) string {
	s := stderr
	s = ansiRegex.ReplaceAllString(s, "")
	s = jwtRegex.ReplaceAllString(s, "[REDACTED-JWT]")
	s = pemRegex.ReplaceAllString(s, "[REDACTED-PEM]")
	s = cookieRegex.ReplaceAllString(s, "[REDACTED-COOKIE]")
	s = urlQueryRegex.ReplaceAllString(s, "[REDACTED-QUERY]")
	s = tokenKVRegex.ReplaceAllString(s, "$1=[REDACTED]")
	s = pathRegex.ReplaceAllString(s, "[REDACTED-PATH]")
	s = highEntropyRegex.ReplaceAllString(s, "[REDACTED]")
	return s
}

var reasonSummaries = map[ErrorReasonCode]struct{ Summary, Hint string }{
	ReasonAuthFailed:            {"Authentication failed", "Check your API key configuration"},
	ReasonQuotaExceeded:         {"Quota exceeded", "Wait and retry, or upgrade your plan"},
	ReasonNetworkError:          {"Network error", "Check your internet connection"},
	ReasonModelNotFound:         {"Model not found", "Verify the model name is correct"},
	ReasonContextWindowExceeded: {"Context window exceeded", "Reduce the conversation length"},
	ReasonServerOverloaded:      {"Server overloaded", "Retry in a few moments"},
	ReasonInvalidConfig:         {"Invalid configuration", "Check your config file"},
	ReasonSessionNotFound:       {"Session not found", "The session may have expired"},
	ReasonToolCallParseFailed:   {"Tool call parse failure", "Check tool call format"},
	ReasonInvalidThinkingSig:    {"Invalid thinking signature", "Internal error, retry the request"},
	ReasonMissingRollout:        {"Missing rollout", "The rollout file was not found"},
	ReasonStallTimeout:          {"CLI stalled (no response)", "The CLI is alive but not producing output; check for long-running operations or kill and retry"},
	ReasonResponseTimeout:       {"Response timeout", "The CLI did not respond in time; retry or increase the timeout"},
	ReasonUpstreamPolicyReject:  {"Upstream policy rejected", "The upstream provider rejected the request per its policy"},
}

// BuildDiagnostics classifies a failure from captured stderr only. See
// BuildDiagnosticsFrom for the dual-source variant (R5).
func BuildDiagnostics(stderr string, exitCode *int, signal string) CliDiagnostics {
	return BuildDiagnosticsFrom(stderr, "", exitCode, signal)
}

// BuildDiagnosticsFrom classifies a CLI failure from one or two sources (R5):
// the captured stderr (priority) and, when stderr is empty, the NDJSON
// stream-level error text recorded during streaming. This mirrors the
// maybeCollectStreamError so that failures which only surface in the stream
// protocol (and not stderr) still get a classified, sanitized diagnosis.
func BuildDiagnosticsFrom(stderr, streamErr string, exitCode *int, signal string) CliDiagnostics {
	d := CliDiagnostics{ExitCode: exitCode, Signal: signal}
	classifySrc := stderr
	excerptSrc := stderr
	if classifySrc == "" {
		classifySrc = streamErr
	}
	if excerptSrc == "" {
		excerptSrc = streamErr
	}
	if excerptSrc == "" {
		return d
	}
	d.ReasonCode = ClassifyError(classifySrc)
	safe := SanitizeStderr(excerptSrc)
	if len(safe) > 200 {
		safe = safe[:200] + "..."
	}
	d.SafeExcerpt = safe
	if info, ok := reasonSummaries[d.ReasonCode]; ok {
		d.PublicSummary = info.Summary
		d.PublicHint = info.Hint
	} else {
		d.PublicSummary = "CLI process failed"
		d.PublicHint = "Check logs for details"
	}
	return d
}

// FormatDiagnostics renders a CliDiagnostics as a human-facing, safe message.
// It never includes raw stderr — only the classified public summary + hint, and
// falls back to the sanitized excerpt (already redacted) only for unclassified
// failures. This is what adapters forward to the client as an error event.
func FormatDiagnostics(d CliDiagnostics) string {
	msg := d.PublicSummary
	if msg == "" {
		msg = "CLI process failed"
	}
	if d.PublicHint != "" {
		msg += "\n提示: " + d.PublicHint
	} else if d.ReasonCode == "" && d.SafeExcerpt != "" {
		msg += ": " + d.SafeExcerpt
	}
	return msg
}

// EmitDiagnosticsIfNeeded waits for the spawned process to exit and, if it
// failed (non-zero exit or a classified stderr reason), sends a single
// sanitized error StreamEvent — unless an error was already surfaced during
// streaming. Adapters call this at the end of their streamEvents goroutine,
// before the channel is closed.
func EmitDiagnosticsIfNeeded(h *SpawnHandle, ch chan<- StreamEvent, sawError bool) {
	h.Wait()
	exitCode, signal := h.ExitInfo()
	diag := BuildDiagnosticsFrom(h.StderrString(), h.streamErrTextSafe(), exitCode, signal)
	if h.stalledFlag() {
		// A stall was observed while the child was running. If it ended in a
		// failure (or without a clear reason), classify as a stall timeout (R7)
		// rather than leaving it unclassified.
		if diag.ReasonCode == "" || (exitCode != nil && *exitCode != 0) {
			diag.ReasonCode = ReasonStallTimeout
			if info, ok := reasonSummaries[ReasonStallTimeout]; ok {
				diag.PublicSummary = info.Summary
				diag.PublicHint = info.Hint
			}
		}
	}
	if (exitCode != nil && *exitCode != 0) || diag.ReasonCode != "" {
		if !sawError {
			// Forward structured diagnostics (cliDiagnostics-style)
			// as event Meta so the client's CliDiagnosticsPanel can render a
			// tier-colored, path-redacted, allowlist-gated panel. The excerpt
			// is already sanitized server-side (REDACTED-*) via
			// BuildDiagnosticsFrom; Source tags where the excerpt came from
			// so the client can gate raw display by an allowlist.
			ch <- StreamEvent{
				Type:    "error",
				Content: FormatDiagnostics(diag),
				Meta: map[string]any{
					"reason":  string(diag.ReasonCode),
					"summary": diag.PublicSummary,
					"hint":    diag.PublicHint,
					"excerpt": diag.SafeExcerpt,
					"source":  "cli_stderr",
				},
			}
		}
	}
}
