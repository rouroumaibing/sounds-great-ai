package telemetry

import (
	"context"
	"testing"
)

func TestWarmupCounters(t *testing.T) {
	cleanup, _ := Init()
	defer cleanup()
	warmupCounters()
	_ = context.Background()
}
