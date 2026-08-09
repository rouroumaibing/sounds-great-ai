package services

import (
	"context"
	"runtime"
	"time"

	opsPorts "sounds-great-ai/internal/domains/ops/ports"
)

// HealthService provides health check and readiness endpoints.
type HealthService struct {
	startTime time.Time
}

// NewHealthService creates a new HealthService.
func NewHealthService(startTime time.Time) *HealthService {
	return &HealthService{startTime: startTime}
}

// Health returns the current health status.
func (s *HealthService) Health(ctx context.Context) (opsPorts.HealthStatus, error) {
	return opsPorts.HealthStatus{
		Status: "ok",
		Uptime: int64(time.Since(s.startTime).Seconds()),
		Checks: map[string]string{"runtime": "ok"},
	}, nil
}

// Ready returns the readiness status with memory stats.
func (s *HealthService) Ready(ctx context.Context) (opsPorts.HealthStatus, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return opsPorts.HealthStatus{
		Status: "ready",
		Uptime: int64(time.Since(s.startTime).Seconds()),
		Checks: map[string]string{
			"runtime":      "ok",
			"heap_objects": string(rune(mem.HeapObjects)),
		},
	}, nil
}

// Uptime returns the service uptime duration.
func (s *HealthService) Uptime() time.Duration {
	return time.Since(s.startTime)
}
