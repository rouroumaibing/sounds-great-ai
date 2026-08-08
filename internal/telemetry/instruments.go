package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	// InvocationDuration — breed invocation duration histogram (ms)
	InvocationDuration metric.Int64Histogram

	// InvocationCompleted — breed invocation completion counter
	// labels: breed, status (ok/error)
	InvocationCompleted metric.Int64Counter

	// TokenUsage — token usage counter
	// labels: breed, type (input/output)
	TokenUsage metric.Int64Counter

	// AgentLiveness — breed liveness gauge
	// 0=dead, 1=idle, 2=busy
	AgentLiveness metric.Int64ObservableGauge

	// A2AHandoffCount — A2A handoff counter
	// labels: from, to
	A2AHandoffCount metric.Int64Counter
)

// warmupCounters pre-touches low-frequency counters so they appear with 0
// in the initial Prometheus scrape.
func warmupCounters() {
	if InvocationCompleted == nil || A2AHandoffCount == nil {
		return
	}
	InvocationCompleted.Add(context.Background(), 0,
		metric.WithAttributes(
			attribute.String("breed", "_warmup"),
			attribute.String("status", "ok"),
		))
	A2AHandoffCount.Add(context.Background(), 0,
		metric.WithAttributes(
			attribute.String("from", "_warmup"),
			attribute.String("to", "_warmup"),
		))
}
