package propagation_test

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/hollis-labs/go-otel/propagation"
)

// A client injects the active trace into a tool call's _meta object; the
// server extracts it from the _meta it receives. Tool arguments are not used.
func ExampleInjectMCPMeta() {
	otel.SetTextMapPropagator(otelpropagation.TraceContext{})

	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	client := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(
		trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled}))

	// Client side: with the go-sdk, params.Meta = mcp.Meta(InjectMCPMeta(ctx, params.Meta)).
	meta := propagation.InjectMCPMeta(client, map[string]any{"hadron/idempotencyKey": "k1"})
	fmt.Println(meta["_traceparent"])

	// Server side: ctx = ExtractMCPMeta(ctx, req.Params.Meta).
	server := propagation.ExtractMCPMeta(context.Background(), meta)
	sc := trace.SpanContextFromContext(server)
	fmt.Println(sc.TraceID(), sc.SpanID(), sc.IsSampled(), meta["hadron/idempotencyKey"])
	// Output:
	// 00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01
	// 0102030405060708090a0b0c0d0e0f10 1112131415161718 true k1
}
