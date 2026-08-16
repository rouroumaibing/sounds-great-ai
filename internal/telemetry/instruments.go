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

	// ProfileUpdateProposed — relationship capsule proposal created
	// (homologous "养熟" governance, KD-10 eval counter).
	ProfileUpdateProposed metric.Int64Counter

	// ProfileUpdateApproved — proposal approved and written to the active capsule.
	ProfileUpdateApproved metric.Int64Counter

	// ProfileUpdateRejected — proposal rejected (not applied).
	ProfileUpdateRejected metric.Int64Counter

	// ProfileDistillationTriggered — distill trigger fired on session seal
	// (KD-10: proves the trigger is wired; zero-activation is "fired N times,
	// processed 0" not "never fired").
	ProfileDistillationTriggered metric.Int64Counter

	// PeopleMemoryProposed / PeopleMemoryApproved / PeopleMemoryRejected are the
	// F276 (People & Relationship Memory) eval counters, kept INDEPENDENT from
	// the F231 relationship-capsule ProfileUpdate* counters so each feature's
	// governance eval is observable on its own (KD-10).
	PeopleMemoryProposed metric.Int64Counter
	PeopleMemoryApproved metric.Int64Counter
	PeopleMemoryRejected metric.Int64Counter

	// PeopleMemoryDrillInvoked — an on-demand recall drill was requested
	// (F276 drill discipline; status label: ok | not_available | budget_exceeded).
	PeopleMemoryDrillInvoked metric.Int64Counter
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
	// Profile-update governance eval counters (homologous KD-10):
	// pre-touch so they appear with value 0 in the first Prometheus scrape
	// (eval can distinguish "fired N times, processed 0" from "never fired").
	if ProfileUpdateProposed != nil {
		ProfileUpdateProposed.Add(context.Background(), 0)
	}
	if ProfileUpdateApproved != nil {
		ProfileUpdateApproved.Add(context.Background(), 0)
	}
	if ProfileUpdateRejected != nil {
		ProfileUpdateRejected.Add(context.Background(), 0)
	}
	if ProfileDistillationTriggered != nil {
		ProfileDistillationTriggered.Add(context.Background(), 0,
			metric.WithAttributes(
				attribute.String("agent.id", "_warmup"),
				attribute.String("seal.reason", "warmup"),
			))
	}
	// F276 people-memory eval counters (KD-10): pre-touch so they appear with
	// value 0 in the first Prometheus scrape (eval distinguishes "fired N times,
	// processed 0" from "never fired").
	if PeopleMemoryProposed != nil {
		PeopleMemoryProposed.Add(context.Background(), 0)
	}
	if PeopleMemoryApproved != nil {
		PeopleMemoryApproved.Add(context.Background(), 0)
	}
	if PeopleMemoryRejected != nil {
		PeopleMemoryRejected.Add(context.Background(), 0)
	}
	if PeopleMemoryDrillInvoked != nil {
		PeopleMemoryDrillInvoked.Add(context.Background(), 0)
	}
}
