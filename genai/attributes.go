// Package genai provides helpers for OpenTelemetry GenAI semantic conventions.
package genai

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys.
const (
	// GenAISystemKey identifies the GenAI system (e.g. "openai", "anthropic").
	GenAISystemKey = attribute.Key("gen_ai.system")

	// GenAIRequestModelKey is the model name used in the request.
	GenAIRequestModelKey = attribute.Key("gen_ai.request.model")

	// GenAIOperationNameKey is the operation performed (e.g. "chat", "completion").
	GenAIOperationNameKey = attribute.Key("gen_ai.operation.name")

	// GenAIUsageInputTokensKey is the number of input tokens consumed.
	GenAIUsageInputTokensKey = attribute.Key("gen_ai.usage.input_tokens")

	// GenAIUsageOutputTokensKey is the number of output tokens produced.
	GenAIUsageOutputTokensKey = attribute.Key("gen_ai.usage.output_tokens")

	// GenAIResponseFinishReasonKey is the finish reason returned by the model.
	GenAIResponseFinishReasonKey = attribute.Key("gen_ai.response.finish_reason")
)
