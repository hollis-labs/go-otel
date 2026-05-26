package hotel

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

// AgentStepSpan creates a "hollis.agent.step" span with the step name as an attribute.
func AgentStepSpan(ctx context.Context, step string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "hollis.agent.step",
		trace.WithAttributes(attribute.String("hollis.agent.step.name", step)),
	)
}

// ToolCallSpan creates a "hollis.tool.call" span with the tool name as an attribute.
func ToolCallSpan(ctx context.Context, tool string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "hollis.tool.call",
		trace.WithAttributes(attribute.String("hollis.tool.name", tool)),
	)
}

// MemoryReadSpan creates a "hollis.memory.read" span.
func MemoryReadSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "hollis.memory.read",
		trace.WithAttributes(
			attribute.String("hollis.memory.namespace", namespace),
			attribute.String("hollis.memory.key", key),
		),
	)
}

// MemoryWriteSpan creates a "hollis.memory.write" span.
func MemoryWriteSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "hollis.memory.write",
		trace.WithAttributes(
			attribute.String("hollis.memory.namespace", namespace),
			attribute.String("hollis.memory.key", key),
		),
	)
}
