package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// DoRequest creates and executes an HTTP request against a handler.
func DoRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// AssertStatus checks the response status code.
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, want, rec.Body.String())
	}
}

// DecodeJSON decodes the response body into v.
func DecodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v; body: %s", err, rec.Body.String())
	}
}

// AssertJSONField checks a top-level JSON field matches expected value.
func AssertJSONField(t *testing.T, rec *httptest.ResponseRecorder, field string, expected any) {
	t.Helper()
	var m map[string]any
	DecodeJSON(t, rec, &m)
	got, ok := m[field]
	if !ok {
		t.Fatalf("field %q not found in response; body: %s", field, rec.Body.String())
	}
	if got != expected {
		t.Errorf("field %q = %v, want %v", field, got, expected)
	}
}
