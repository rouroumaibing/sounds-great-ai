package telemetry

import (
	"fmt"
	"testing"
	"time"
)

func TestTraceStore_Add_Query(t *testing.T) {
	ts := NewTraceStore(100)
	ts.Add(Span{TraceID: "t1", SpanID: "s1", Name: "test", StartTime: time.Now()})
	results := ts.Query("t1", "", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestTraceStore_Capacity(t *testing.T) {
	ts := NewTraceStore(3)
	for i := 0; i < 5; i++ {
		ts.Add(Span{TraceID: fmt.Sprintf("t%d", i), StartTime: time.Now()})
	}
	stats := ts.Stats()
	if stats.Count != 3 {
		t.Fatalf("expected 3 retained, got %d", stats.Count)
	}
	if len(ts.Query("t0", "", 10)) != 0 {
		t.Fatal("expected t0 evicted")
	}
	if len(ts.Query("t4", "", 10)) != 1 {
		t.Fatal("expected t4 retained")
	}
}

func TestTraceStore_Query_ByBreed(t *testing.T) {
	ts := NewTraceStore(100)
	ts.Add(Span{TraceID: "t1", Attributes: map[string]any{"breed": "bianmu"}, StartTime: time.Now()})
	ts.Add(Span{TraceID: "t2", Attributes: map[string]any{"breed": "xigou"}, StartTime: time.Now()})
	if got := len(ts.Query("", "bianmu", 10)); got != 1 {
		t.Fatalf("expected 1 bianmu span, got %d", got)
	}
}

func TestTraceStore_Query_LimitClamp(t *testing.T) {
	ts := NewTraceStore(100)
	for i := 0; i < 50; i++ {
		ts.Add(Span{TraceID: "t", StartTime: time.Now()})
	}
	if got := len(ts.Query("", "", 5)); got != 5 {
		t.Fatalf("expected 5 results, got %d", got)
	}
	if got := len(ts.Query("", "", 10000)); got != 50 {
		t.Fatalf("expected clamp to 50, got %d", got)
	}
}

func TestTraceStore_Stats(t *testing.T) {
	ts := NewTraceStore(10)
	if s := ts.Stats(); s.Count != 0 || s.MaxSize != 10 {
		t.Fatalf("unexpected stats: %+v", s)
	}
}
