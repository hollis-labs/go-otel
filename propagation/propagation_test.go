package propagation

import (
	"context"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectAndExtractMCP(t *testing.T) {
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{},
		otelpropagation.Baggage{},
	))

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

	params := InjectMCP(ctx, nil)
	if got, want := params["_traceparent"], "00-"+traceID.String()+"-"+spanID.String()+"-01"; got != want {
		t.Fatalf("_traceparent = %v, want %v", got, want)
	}

	gotCtx := ExtractMCP(params)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if gotSC.TraceID() != traceID {
		t.Fatalf("traceID = %s, want %s", gotSC.TraceID(), traceID)
	}
	if gotSC.SpanID() != spanID {
		t.Fatalf("spanID = %s, want %s", gotSC.SpanID(), spanID)
	}
}

func TestInjectHTTPAddsTraceContext(t *testing.T) {
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{},
		otelpropagation.Baggage{},
	))

	var traceID trace.TraceID
	copy(traceID[:], []byte{
		0xaa, 0xbb, 0xcc, 0xdd,
		0xee, 0xff, 0x00, 0x11,
		0x22, 0x33, 0x44, 0x55,
		0x66, 0x77, 0x88, 0x99,
	})
	var spanID trace.SpanID
	copy(spanID[:], []byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	InjectHTTP(ctx, req)

	if got, want := req.Header.Get("traceparent"), "00-"+traceID.String()+"-"+spanID.String()+"-01"; got != want {
		t.Fatalf("traceparent = %q, want %q", got, want)
	}
}
