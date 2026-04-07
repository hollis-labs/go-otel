---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Architecture

## Tech stack

| Component | Version |
|-----------|---------|
| Go | 1.25.0 |
| OTel SDK (trace) | v1.41.0 |
| OTel SDK (metric) | v1.41.0 |
| OTel OTLP HTTP exporter | v1.41.0 |
| OTel stdout exporter | v1.41.0 |

## Package layout

```
github.com/hollis-labs/otel
  tiamatotel.go        -- Init(), Option funcs (WithServiceName, WithEnvironment, etc.)
  tracing.go           -- Span helpers: StartSpan, AgentStepSpan, ToolCallSpan, MemoryReadSpan, MemoryWriteSpan
  metrics.go           -- RegisterMetrics() -> TiamatMetrics (request count, latency, error count)
  logging.go           -- NewLogHandler() wraps slog.Handler for trace_id/span_id injection
  genai/
    attributes.go      -- GenAI semconv attribute key constants
    genai.go           -- ModelCallSpan, RecordTokenUsage, RecordModelLatency
  propagation/
    propagation.go     -- HTTPMiddleware, InjectHTTP, ExtractMCP, InjectMCP
  redaction/
    redaction.go       -- SpanProcessor (denylist-based), ShouldRedact, Denylist()
  internal/
    resource.go        -- NewResource() builds OTel Resource with Tiamat attributes
  examples/hello/
    main.go            -- Example consumer
```

## Exported API surface

### Root package (`tiamatotel`)

- `Init(ctx, ...Option) (shutdown func, err)` — bootstraps trace provider with OTLP HTTP exporter
- `WithServiceName(string) Option`
- `WithServiceVersion(string) Option`
- `WithEnvironment(string) Option`
- `WithOTLPEndpoint(string) Option`
- `WithSampler(sdktrace.Sampler) Option`
- `StartSpan(ctx, name, ...SpanStartOption) (ctx, Span)`
- `AgentStepSpan(ctx, step) (ctx, Span)`
- `ToolCallSpan(ctx, tool) (ctx, Span)`
- `MemoryReadSpan(ctx, namespace, key) (ctx, Span)`
- `MemoryWriteSpan(ctx, namespace, key) (ctx, Span)`
- `RegisterMetrics(meter) (*TiamatMetrics, error)`
- `NewLogHandler(slog.Handler) slog.Handler`

### `genai` sub-package

- `ModelCallSpan(ctx, model, operation) (ctx, Span)`
- `RecordTokenUsage(span, inputTokens, outputTokens)`
- `RecordModelLatency(ctx, model, duration)`
- Attribute key constants: `GenAISystemKey`, `GenAIRequestModelKey`, `GenAIOperationNameKey`, `GenAIUsageInputTokensKey`, `GenAIUsageOutputTokensKey`, `GenAIResponseFinishReasonKey`

### `propagation` sub-package

- `HTTPMiddleware(http.Handler) http.Handler`
- `InjectHTTP(ctx, *http.Request)`
- `ExtractMCP(map[string]interface{}) context.Context`
- `InjectMCP(ctx, map[string]interface{}) map[string]interface{}`

### `redaction` sub-package

- `SpanProcessor() sdktrace.SpanProcessor`
- `Denylist() []string`

## Consumer integration

All consuming Go projects use a `replace` directive in their `go.mod`:

```
require github.com/hollis-labs/otel v0.0.0
replace github.com/hollis-labs/otel => ../tiamat-otel
```

This allows local development without publishing to a registry.
