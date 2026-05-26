package hotel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// NewLogHandler wraps an slog.Handler to inject trace_id and span_id from
// the context into every log record. It does NOT push log records over
// OTLP; pair with NewSlogHandler if you also want OTLP log export under
// Init's WithLogsEnabled.
func NewLogHandler(inner slog.Handler) slog.Handler {
	return &traceLogHandler{inner: inner}
}

// NewSlogHandler returns an slog.Handler that fans out every log record to
// two destinations:
//
//   - stderrInner wrapped by NewLogHandler — preserves the existing stderr
//     pretty-print path with trace_id / span_id injection.
//   - The OTel slog bridge bound to scopeName — emits records through the
//     OTel log API to whatever LoggerProvider is currently installed. When
//     Init was called with WithLogsEnabled, that's the OTLP exporter set
//     up there; otherwise it's the OTel no-op LoggerProvider and the
//     bridge silently discards records.
//
// scopeName is the instrumentation-scope name attached to OTLP log
// records; convention is the importing package path (e.g.
// "github.com/hollis-labs/torque").
//
// To opt out of the stderr fan-out entirely (OTLP-only), construct the
// bridge directly via otelslog.NewHandler from
// go.opentelemetry.io/contrib/bridges/otelslog.
func NewSlogHandler(scopeName string, stderrInner slog.Handler) slog.Handler {
	return &fanoutHandler{
		handlers: []slog.Handler{
			NewLogHandler(stderrInner),
			otelslog.NewHandler(scopeName),
		},
	}
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

// fanoutHandler dispatches each record to every wrapped handler, joining
// any per-handler errors so a failure in one path doesn't suppress the
// others.
type fanoutHandler struct {
	handlers []slog.Handler
}

func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, inner := range h.handlers {
		if inner.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, inner := range h.handlers {
		// Clone per handler — handlers may add attrs (trace_id, etc.) and
		// we don't want one path's mutation to leak into the next.
		if err := inner.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, inner := range h.handlers {
		next[i] = inner.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, inner := range h.handlers {
		next[i] = inner.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
