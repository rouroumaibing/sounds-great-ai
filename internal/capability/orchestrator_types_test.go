package capability

import (
	"testing"
)

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

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	if got := truncateRunes("hello world", 5); got != "hello" {
		t.Errorf("truncateRunes() = %q, want %q", got, "hello")
	}
	if got := truncateRunes("你好世界", 2); got != "你好" {
		t.Errorf("truncateRunes() UTF-8 = %q, want %q", got, "你好")
	}
	if got := truncateRunes("short", 10); got != "short" {
		t.Errorf("truncateRunes() no truncation = %q, want %q", got, "short")
	}
	if got := truncateRunes("abc", 0); got != "" {
		t.Errorf("truncateRunes() max=0 = %q, want empty", got)
	}
}
