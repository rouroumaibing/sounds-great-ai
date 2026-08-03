package capability

import (
	"strings"
	"testing"
)

func TestSanitizeJSONResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain JSON", `[{"a":1}]`, `[{"a":1}]`},
		{"markdown wrapped", "```json\n[{\"a\":1}]\n```", `[{"a":1}]`},
		{"markdown no lang", "```\n[{\"a\":1}]\n```", `[{"a":1}]`},
		{"leading text", "Here is the result:\n```json\n[{\"a\":1}]\n```\nDone.", `[{"a":1}]`},
		{"object fallback", `prefix {"key":"val"} suffix`, `{"key":"val"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeJSONResponse(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeJSONResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBalancedTruncate(t *testing.T) {
	t.Parallel()
	results := map[string]string{
		"breed_a": strings.Repeat("a", 5000),
		"breed_b": strings.Repeat("b", 5000),
	}
	truncated := balancedTruncate(results, 2000)
	if len([]rune(truncated["breed_a"])) > 1000+20 { // perBreed=1000 + truncation marker
		t.Errorf("breed_a not truncated: len=%d", len(truncated["breed_a"]))
	}
	if len([]rune(truncated["breed_b"])) > 1000+20 {
		t.Errorf("breed_b not truncated: len=%d", len(truncated["breed_b"]))
	}
}

func TestBalancedTruncateUTF8(t *testing.T) {
	t.Parallel()
	// Chinese characters are 3 bytes each in UTF-8
	chinese := strings.Repeat("中", 100) // 300 bytes, 100 runes
	results := map[string]string{"breed": chinese}
	truncated := balancedTruncate(results, 50) // perBreed=50 runes
	result := truncated["breed"]
	if !strings.HasPrefix(result, "中") {
		t.Errorf("UTF-8 truncation produced invalid output: %q", result)
	}
	// Should be valid UTF-8 (no replacement chars)
	if strings.Contains(result, "\ufffd") {
		t.Errorf("UTF-8 truncation produced replacement char: %q", result)
	}
}

func TestGetIntConfig(t *testing.T) {
	t.Parallel()
	cfg := map[string]any{"max_depth": float64(5)}
	if got := getIntConfig(cfg, "max_depth", 3); got != 5 {
		t.Errorf("getIntConfig() = %d, want 5", got)
	}
	if got := getIntConfig(nil, "max_depth", 3); got != 3 {
		t.Errorf("getIntConfig(nil) = %d, want 3", got)
	}
	if got := getIntConfig(cfg, "missing", 3); got != 3 {
		t.Errorf("getIntConfig(missing) = %d, want 3", got)
	}
}

func TestDecodeData(t *testing.T) {
	t.Parallel()
	src := []any{
		map[string]any{"id": "sub-1", "title": "Task A", "description": "Do A", "suggest_breed": "xigou"},
	}
	var dst []SubTask
	if err := decodeData(src, &dst); err != nil {
		t.Fatalf("decodeData: %v", err)
	}
	if len(dst) != 1 || dst[0].Title != "Task A" {
		t.Errorf("decodeData result = %+v", dst)
	}
}
