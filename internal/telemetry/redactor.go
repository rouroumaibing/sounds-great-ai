package telemetry

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
