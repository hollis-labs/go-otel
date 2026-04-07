package feotel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NewLogHandler wraps an slog.Handler to inject trace_id and span_id
// from the context into every log record.
func NewLogHandler(inner slog.Handler) slog.Handler {
	return &traceLogHandler{inner: inner}
}

type traceLogHandler struct {
	inner slog.Handler
}

func (h *traceLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceLogHandler) Handle(ctx context.Context, record slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		record.AddAttrs(slog.String("trace_id", sc.TraceID().String()))
	}
	if sc.HasSpanID() {
		record.AddAttrs(slog.String("span_id", sc.SpanID().String()))
	}
	return h.inner.Handle(ctx, record)
}

func (h *traceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceLogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceLogHandler) WithGroup(name string) slog.Handler {
	return &traceLogHandler{inner: h.inner.WithGroup(name)}
}
