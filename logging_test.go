package hotel

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

type captureHandler struct {
	attrs []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		h.attrs = append(h.attrs, attr)
		return true
	})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := &captureHandler{}
	next.attrs = append(next.attrs, attrs...)
	return next
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func TestNewLogHandlerInjectsTraceAndSpanIDs(t *testing.T) {
	var traceID trace.TraceID
	copy(traceID[:], []byte{
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c,
		0x0d, 0x0e, 0x0f, 0x10,
	})
	var spanID trace.SpanID
	copy(spanID[:], []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	capture := &captureHandler{}
	logger := slog.New(NewLogHandler(capture))
	logger.InfoContext(ctx, "hello", slog.String("component", "test"))

	var gotTraceID, gotSpanID string
	for _, attr := range capture.attrs {
		switch attr.Key {
		case "trace_id":
			gotTraceID = attr.Value.String()
		case "span_id":
			gotSpanID = attr.Value.String()
		}
	}

	if gotTraceID != traceID.String() {
		t.Fatalf("trace_id = %q, want %q", gotTraceID, traceID.String())
	}
	if gotSpanID != spanID.String() {
		t.Fatalf("span_id = %q, want %q", gotSpanID, spanID.String())
	}
}
