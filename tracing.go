package feotel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/hollis-labs/go-otel"

// StartSpan creates a span using the global tracer provider.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, opts...)
}

// AgentStepSpan creates a "fe.agent.step" span with the step name as an attribute.
func AgentStepSpan(ctx context.Context, step string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "fe.agent.step",
		trace.WithAttributes(attribute.String("fe.agent.step.name", step)),
	)
}

// ToolCallSpan creates a "fe.tool.call" span with the tool name as an attribute.
func ToolCallSpan(ctx context.Context, tool string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "fe.tool.call",
		trace.WithAttributes(attribute.String("fe.tool.name", tool)),
	)
}

// MemoryReadSpan creates a "fe.memory.read" span.
func MemoryReadSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "fe.memory.read",
		trace.WithAttributes(
			attribute.String("fe.memory.namespace", namespace),
			attribute.String("fe.memory.key", key),
		),
	)
}

// MemoryWriteSpan creates a "fe.memory.write" span.
func MemoryWriteSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "fe.memory.write",
		trace.WithAttributes(
			attribute.String("fe.memory.namespace", namespace),
			attribute.String("fe.memory.key", key),
		),
	)
}
