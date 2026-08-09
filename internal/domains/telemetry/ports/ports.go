package ports

import (
	"context"
	"time"
)

// TraceSpan represents a telemetry trace span.
type TraceSpan struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Attributes map[string]string     `json:"attributes,omitempty"`
}

// ITraceStore is the port for trace storage.
type ITraceStore interface {
	Record(ctx context.Context, span TraceSpan) error
	Query(ctx context.Context, filter TraceFilter) ([]TraceSpan, error)
}

// TraceFilter filters trace queries.
type TraceFilter struct {
	Name  string
	From  time.Time
	To    time.Time
	Limit int
}

// IMetricsCollector is the port for metrics collection.
type IMetricsCollector interface {
	Increment(ctx context.Context, name string, tags map[string]string) error
	Gauge(ctx context.Context, name string, value float64, tags map[string]string) error
}
