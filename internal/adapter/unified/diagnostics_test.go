package unified

import (
	"strings"
	"testing"
)

func TestClassifyError_AuthFailed(t *testing.T) {
	code := ClassifyError("Error: Invalid API key")
	if code != ReasonAuthFailed {
		t.Errorf("got %q, want %q", code, ReasonAuthFailed)
	}
}

func TestClassifyError_QuotaExceeded(t *testing.T) {
	code := ClassifyError("Error: You have exceeded your quota")
	if code != ReasonQuotaExceeded {
		t.Errorf("got %q, want %q", code, ReasonQuotaExceeded)
	}
}

func TestClassifyError_NetworkError(t *testing.T) {
	code := ClassifyError("Error: connect ECONNREFUSED 127.0.0.1:443")
	if code != ReasonNetworkError {
		t.Errorf("got %q, want %q", code, ReasonNetworkError)
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	code := ClassifyError("some random error text")
	if code != "" {
		t.Errorf("got %q, want empty", code)
	}
}

func TestSanitizeStderr_RemovesANSI(t *testing.T) {
	input := "\x1b[31mError\x1b[0m: something"
	out := SanitizeStderr(input)
	if strings.Contains(out, "\x1b") {
		t.Errorf("ANSI not removed: %q", out)
	}
}

func TestSanitizeStderr_RemovesJWT(t *testing.T) {
	input := "token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123def456ghi789"
	out := SanitizeStderr(input)
	if strings.Contains(out, "eyJhbGci") {
		t.Errorf("JWT not redacted: %q", out)
	}
}

func TestSanitizeStderr_RemovesPaths(t *testing.T) {
	input := "Error at /Users/john/.config/claude/config.json"
	out := SanitizeStderr(input)
	if strings.Contains(out, "/Users/john") {
		t.Errorf("Path not redacted: %q", out)
	}
}

func TestBuildDiagnostics(t *testing.T) {
	code := 1
	d := BuildDiagnostics("Error: Invalid API key", &code, "")
	if d.ReasonCode != ReasonAuthFailed {
		t.Errorf("ReasonCode = %q, want %q", d.ReasonCode, ReasonAuthFailed)
	}
	if d.PublicSummary == "" {
		t.Error("PublicSummary should not be empty")
	}
	if strings.Contains(d.PublicSummary, "Invalid API key") {
		t.Error("PublicSummary should not contain raw stderr")
	}
}

func TestBuildDiagnostics_EmptyStderr(t *testing.T) {
	code := 1
	d := BuildDiagnostics("", &code, "")
	if d.ReasonCode != "" {
		t.Errorf("ReasonCode = %q, want empty", d.ReasonCode)
	}
}
