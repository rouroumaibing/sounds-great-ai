package telemetry

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker, delegating to the underlying ResponseWriter.
// Required for WebSocket upgrade to work through TraceMiddleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func statusStr(code int) string {
	if code >= 400 {
		return "error"
	}
	return "ok"
}

// TraceMiddleware creates a span for each HTTP request and writes it to TraceStore.
// No-op (passthrough) when telemetry is not initialized.
func TraceMiddleware(next http.Handler) http.Handler {
	if !initialized {
		return next
	}
	tracer := otel.Tracer("sounds-great-ai")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r.WithContext(ctx))
		duration := time.Since(start)

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
			attribute.Int("http.status", ww.status),
			attribute.Int64("http.duration_ms", duration.Milliseconds()),
		)
		if ww.status >= 400 {
			span.SetStatus(codes.Error, "")
		}

		if globalTraceStore != nil {
			s := Span{
				TraceID:   span.SpanContext().TraceID().String(),
				SpanID:    span.SpanContext().SpanID().String(),
				Name:      r.Method + " " + r.URL.Path,
				StartTime: start,
				EndTime:   time.Now(),
				Attributes: map[string]any{
					"http.method":  r.Method,
					"http.path":    r.URL.Path,
					"http.status":  ww.status,
					"http.duration": duration.Milliseconds(),
				},
				Status: statusStr(ww.status),
			}
			if globalRedactor != nil {
				globalRedactor.RedactSpan(&s)
			}
			globalTraceStore.Add(s)
		}
	})
}
