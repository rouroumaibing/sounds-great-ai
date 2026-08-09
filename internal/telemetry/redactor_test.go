package telemetry

import "testing"

func TestRedactor_Pseudonymize_Determinism(t *testing.T) {
	r := NewRedactor("test-salt")
	a := r.Pseudonymize("thread-123")
	b := r.Pseudonymize("thread-123")
	if a != b {
		t.Fatalf("expected deterministic output, got %q vs %q", a, b)
	}
}

func TestRedactor_Pseudonymize_DifferentInputs(t *testing.T) {
	r := NewRedactor("test-salt")
	if r.Pseudonymize("a") == r.Pseudonymize("b") {
		t.Fatal("expected different pseudonyms for different inputs")
	}
}

func TestRedactor_Pseudonymize_Length(t *testing.T) {
	r := NewRedactor("test-salt")
	if got := len(r.Pseudonymize("x")); got != 16 {
		t.Fatalf("expected 16-char pseudonym, got %d", got)
	}
}

func TestRedactor_RedactSpan(t *testing.T) {
	r := NewRedactor("test-salt")
	span := Span{Attributes: map[string]any{"threadID": "secret-123", "breed": "bianmu"}}
	r.RedactSpan(&span)
	if span.Attributes["threadID"] == "secret-123" {
		t.Fatal("expected threadID to be pseudonymized")
	}
	if span.Attributes["breed"] != "bianmu" {
		t.Fatal("expected non-sensitive attribute to be unchanged")
	}
}

func TestRedactor_RedactSpan_NilAttributes(t *testing.T) {
	r := NewRedactor("test-salt")
	span := Span{}
	r.RedactSpan(&span) // should not panic
}
