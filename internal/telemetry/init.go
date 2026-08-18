package telemetry

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelProm "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

var (
	globalTraceStore    *TraceStore
	globalSnapshotStore *SnapshotStore
	globalRedactor      *Redactor
	globalMeterProvider *metricSdk.MeterProvider
	globalPromHandler   http.Handler
	initialized         bool
	startTime           = time.Now()
	initMu              sync.Mutex
)

// Init initializes the OpenTelemetry SDK. On failure, logs a warning and
// returns an empty cleanup — never panics (graceful degradation).
func Init() (func(), error) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		return func() {}, nil
	}

	cleanup, err := initOTel()
	if err != nil {
		log.Printf("[telemetry] init failed, running without telemetry: %v", err)
		return func() {}, nil // graceful degradation
	}
	return cleanup, nil
}

func initOTel() (func(), error) {
	// Prometheus registry + exporter
	reg := prometheus.NewRegistry()
	exporter, err := otelProm.New(otelProm.WithRegisterer(reg))
	if err != nil {
		return func() {}, err
	}
	provider := metricSdk.NewMeterProvider(metricSdk.WithReader(exporter))
	otel.SetMeterProvider(provider)

	// Tracer provider
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	// Register instruments
	meter := provider.Meter("sounds-great-ai")
	InvocationDuration, _ = meter.Int64Histogram("dog_pack.invocation.duration",
		metric.WithUnit("ms"))
	InvocationCompleted, _ = meter.Int64Counter("dog_pack.invocation.completed")
	TokenUsage, _ = meter.Int64Counter("dog_pack.token.usage")
	A2AHandoffCount, _ = meter.Int64Counter("dog_pack.a2a.handoff.count")
	// Profile-update governance eval counters (homologous KD-10).
	// Counter names follow the dog_pack.profile_update.* convention,
	// under the SG dog_pack. namespace.
	ProfileUpdateProposed, _ = meter.Int64Counter("dog_pack.profile_update.proposed")
	ProfileUpdateApproved, _ = meter.Int64Counter("dog_pack.profile_update.approved")
	ProfileUpdateRejected, _ = meter.Int64Counter("dog_pack.profile_update.rejected")
	ProfileDistillationTriggered, _ = meter.Int64Counter("dog_pack.profile_update.distillation_triggered")
	// F276 People & Relationship Memory eval counters (independent of F231
	// capsule's profile_update.* counters — KD-10 governance observability).
	PeopleMemoryProposed, _ = meter.Int64Counter("dog_pack.people_memory.proposed")
	PeopleMemoryApproved, _ = meter.Int64Counter("dog_pack.people_memory.approved")
	PeopleMemoryRejected, _ = meter.Int64Counter("dog_pack.people_memory.rejected")
	PeopleMemoryDrillInvoked, _ = meter.Int64Counter("dog_pack.people_memory.drill_invoked")
	// Shared Memory (typed-lane) governance eval counters (homologous clowder
	// CrossCatMetricsComputer / F200 / F263). Per-lane candidate/approve/reject
	// lifecycle + per-breed truth injection.
	LaneCandidateSubmitted, _ = meter.Int64Counter("dog_pack.lane.candidate_submitted")
	LaneApproved, _ = meter.Int64Counter("dog_pack.lane.approved")
	LaneRejected, _ = meter.Int64Counter("dog_pack.lane.rejected")
	LaneRetired, _ = meter.Int64Counter("dog_pack.lane.retired")
	LaneForgotten, _ = meter.Int64Counter("dog_pack.lane.forgotten")
	LaneDeferred, _ = meter.Int64Counter("dog_pack.lane.deferred")
	LaneUndone, _ = meter.Int64Counter("dog_pack.lane.undone")
	LaneWithdrawn, _ = meter.Int64Counter("dog_pack.lane.withdrawn")
	LaneTruthInjected, _ = meter.Int64Counter("dog_pack.lane.truth_injected")
	warmupCounters()

	// Global stores
	globalTraceStore = NewTraceStore(10000)
	salt := os.Getenv("OTEL_REDACT_SALT")
	globalRedactor = NewRedactor(salt)

	// Prometheus HTTP handler
	globalPromHandler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	// Prometheus HTTP endpoint (port configurable, graceful on conflict)
	promPort := os.Getenv("OTEL_PROM_PORT")
	if promPort == "" {
		promPort = "9464"
	}
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", globalPromHandler)
		srv := &http.Server{Addr: ":" + promPort, Handler: mux}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[telemetry] prometheus exporter on :%s failed: %v", promPort, err)
		}
	}()

	// SnapshotStore: samples Prometheus text every 30s
	collect := func() string { return collectPromText() }
	globalSnapshotStore = NewSnapshotStore(720, 30*time.Second, collect)
	globalSnapshotStore.Start(context.Background())

	initialized = true
	cleanup := func() {
		_ = tp.Shutdown(context.Background())
		_ = provider.Shutdown(context.Background())
	}
	return cleanup, nil
}

// Shutdown gracefully shuts down all telemetry resources.
func Shutdown() error {
	if globalMeterProvider != nil {
		return globalMeterProvider.Shutdown(context.Background())
	}
	return nil
}

// IsInitialized returns whether telemetry was successfully initialized.
func IsInitialized() bool { return initialized }

// TraceStoreInstance returns the global TraceStore (nil if not initialized).
func TraceStoreInstance() *TraceStore { return globalTraceStore }

// SnapshotStoreInstance returns the global SnapshotStore (nil if not initialized).
func SnapshotStoreInstance() *SnapshotStore { return globalSnapshotStore }

// RedactorInstance returns the global Redactor (nil if not initialized).
func RedactorInstance() *Redactor { return globalRedactor }

// StartTime returns the process start time (for health endpoint uptime).
func StartTime() time.Time { return startTime }

// PromHandler returns the Prometheus HTTP handler (nil if not initialized).
func PromHandler() http.Handler { return globalPromHandler }

// collectPromText extracts Prometheus text format from the global handler.
func collectPromText() string {
	if globalPromHandler == nil {
		return ""
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	globalPromHandler.ServeHTTP(rec, req)
	return rec.Body.String()
}
