package capability

import (
	"math"
	"testing"
)

func TestGetFloatConfig_Float64(t *testing.T) {
	cfg := map[string]any{"threshold": 0.75}
	if got := getFloatConfig(cfg, "threshold", 0.0); math.Abs(got-0.75) > 1e-9 {
		t.Fatalf("want 0.75, got %v", got)
	}
}

func TestGetFloatConfig_Int(t *testing.T) {
	// JSON may parse 0 as int in some paths
	cfg := map[string]any{"threshold": 0}
	if got := getFloatConfig(cfg, "threshold", 0.5); got != 0.0 {
		t.Fatalf("want 0.0, got %v", got)
	}
}

func TestGetFloatConfig_Int64(t *testing.T) {
	cfg := map[string]any{"threshold": int64(1)}
	if got := getFloatConfig(cfg, "threshold", 0.0); got != 1.0 {
		t.Fatalf("want 1.0, got %v", got)
	}
}

func TestGetFloatConfig_Missing(t *testing.T) {
	cfg := map[string]any{}
	if got := getFloatConfig(cfg, "threshold", 0.5); got != 0.5 {
		t.Fatalf("want default 0.5, got %v", got)
	}
}

func TestGetFloatConfig_NilCfg(t *testing.T) {
	if got := getFloatConfig(nil, "threshold", 0.5); got != 0.5 {
		t.Fatalf("want default 0.5, got %v", got)
	}
}

func TestGetFloatConfig_InvalidType(t *testing.T) {
	cfg := map[string]any{"threshold": "not a number"}
	if got := getFloatConfig(cfg, "threshold", 0.5); got != 0.5 {
		t.Fatalf("want default 0.5, got %v", got)
	}
}
