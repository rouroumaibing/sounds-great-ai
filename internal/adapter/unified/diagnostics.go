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
}

func BuildDiagnostics(stderr string, exitCode *int, signal string) CliDiagnostics {
	d := CliDiagnostics{ExitCode: exitCode, Signal: signal}
	if stderr == "" {
		return d
	}
	d.ReasonCode = ClassifyError(stderr)
	safe := SanitizeStderr(stderr)
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
