package genai

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/hollis-labs/go-otel/genai"
const meterName = "github.com/hollis-labs/go-otel/genai"

// ModelCallSpan creates a span following OTel GenAI semantic conventions.
func ModelCallSpan(ctx context.Context, model, operation string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, "gen_ai."+operation,
		trace.WithAttributes(
			attribute.String(string(GenAIRequestModelKey), model),
			attribute.String(string(GenAIOperationNameKey), operation),
		),
	)
}

// RecordTokenUsage sets token usage attributes on the given span.
func RecordTokenUsage(span trace.Span, inputTokens, outputTokens int) {
	span.SetAttributes(
		attribute.Int(string(GenAIUsageInputTokensKey), inputTokens),
		attribute.Int(string(GenAIUsageOutputTokensKey), outputTokens),
	)
}

// RecordModelLatency records model call latency to a histogram metric.
func RecordModelLatency(ctx context.Context, model string, duration time.Duration) {
	histogram, err := otel.Meter(meterName).Float64Histogram(
		"gen_ai.client.operation.duration",
		metric.WithDescription("GenAI model call latency"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return
	}
	histogram.Record(ctx, float64(duration.Milliseconds()),
		metric.WithAttributes(attribute.String(string(GenAIRequestModelKey), model)),
	)
}
