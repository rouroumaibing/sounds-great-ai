package telemetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

type Redactor struct {
	salt string
}

// NewRedactor creates a Redactor. salt should come from env OTEL_REDACT_SALT.
func NewRedactor(salt string) *Redactor {
	if salt == "" {
		salt = "dev-only-salt" // only for dev; production must inject
	}
	return &Redactor{salt: salt}
}

// Pseudonymize uses HMAC-SHA256(salt, id) to produce a deterministic
// pseudonymous ID. Same salt + same id always yields the same output.
// Returns the first 16 hex characters.
func (r *Redactor) Pseudonymize(id string) string {
	mac := hmac.New(sha256.New, []byte(r.salt))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// sensitiveKeys are attribute keys that must be pseudonymized.
var sensitiveKeys = []string{"threadID", "invocationID"}

// RedactSpan modifies span attributes in-place, pseudonymizing sensitive keys.
func (r *Redactor) RedactSpan(span *Span) {
	if span.Attributes == nil {
		return
	}
	for _, key := range sensitiveKeys {
		if val, ok := span.Attributes[key]; ok {
			if id, ok := val.(string); ok {
				span.Attributes[key] = r.Pseudonymize(id)
			}
		}
	}
}

// secretPatterns match common credential shapes that must never cross a breed
// (handoff) boundary in plaintext. Keys are preserved; values are masked.
var secretPatterns = []*regexp.Regexp{
	// key = value / "value"  forms (api_key, secret, token, password, ...)
	regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password|passwd|pwd|access[_-]?token|auth[_-]?token|client[_-]?secret)\s*[:=]\s*["']?)([^\s"',}{]+)`),
	// provider-style literal tokens
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9]{16,}\b`),
	regexp.MustCompile(`(?i)\bghp_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._\-]{12,})\b`),
}

// RedactSecrets masks credential values in arbitrary text. It reuses the global
// Redactor's HMAC salt so the same secret always maps to the same 16-hex mask
// (deterministic, reversible only with the salt). Empty/missing salt falls back
// to a fixed "***" mask. Safe to call on any text; non-matches pass through.
func RedactSecrets(text string) string {
	r := RedactorInstance()
	for _, p := range secretPatterns {
		text = p.ReplaceAllStringFunc(text, func(match string) string {
			subs := p.FindStringSubmatch(match)
			if len(subs) < 3 {
				// Whole-match pattern (sk-/ghp-/Bearer): mask the entire token.
				if r != nil {
					return r.Pseudonymize(match)
				}
				return "***"
			}
			// Key = value form: keep the key prefix, mask the value.
			prefix, value := subs[1], subs[2]
			mask := "***"
			if r != nil {
				mask = r.Pseudonymize(value)
			}
			return prefix + mask
		})
	}
	return text
}
