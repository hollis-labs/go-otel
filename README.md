# Fragments Engine OpenTelemetry (feotel)

`feotel` is the shared OpenTelemetry instrumentation library for the Fragments Engine suite of applications (Engine, Conduit, Cortex, Hadron, Nanite). It provides a single entry point for initializing a trace provider with an OTLP HTTP exporter, helpers for the standard Fragments Engine span taxonomy, GenAI semantic-convention emitters, W3C trace context propagation for HTTP and MCP, an `slog` handler that injects trace correlation, and a redaction denylist helper for sensitive prompt content.

## Status

Beta. The public API has a coherent shape and is exercised by an example program and focused tests, but the module still has no `CHANGELOG.md`, so API churn cannot be ruled out. Used as a shared dependency across the Fragments Engine apps per `docs/observability-contract.md`.

## Install

```bash
go get github.com/hollis-labs/otel
```

## Usage

Minimal initialization in an application `main()`:

```go
package main

import (
    "context"
    "log"

    feotel "github.com/hollis-labs/otel"
)

func main() {
    ctx := context.Background()

    shutdown, err := feotel.Init(ctx,
        feotel.WithServiceName("my-service"),
        feotel.WithServiceVersion("0.1.0"),
        feotel.WithEnvironment("development"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

    ctx, span := feotel.StartSpan(ctx, "my.operation")
    defer span.End()

    // ... application work ...
}
```

A runnable example using the stdout exporter (no collector required) lives at `examples/hello/main.go` and demonstrates the core span helpers and the `genai` sub-package.

## API Overview

Top-level package `feotel` (`github.com/hollis-labs/otel`):

- `Init(ctx, opts...) (shutdown func(context.Context) error, err error)` — installs an OTLP HTTP trace provider and W3C+Baggage propagators.
- `Option` and option constructors: `WithServiceName`, `WithServiceVersion`, `WithEnvironment`, `WithOTLPEndpoint` (default `localhost:4318`), `WithSampler`.
- `StartSpan(ctx, name, opts...)` — wraps the global tracer for ad-hoc spans.
- `AgentStepSpan(ctx, step)` — `fe.agent.step` span with `fe.agent.step.name`.
- `ToolCallSpan(ctx, tool)` — `fe.tool.call` span with `fe.tool.name`.
- `MemoryReadSpan(ctx, namespace, key)` / `MemoryWriteSpan(ctx, namespace, key)` — `fe.memory.read` / `fe.memory.write` spans.
- `RegisterMetrics(meter) (*Metrics, error)` — registers `fe.request.count`, `fe.request.latency`, `fe.error.count` instruments.
- `NewLogHandler(inner slog.Handler) slog.Handler` — wraps an `slog.Handler` to inject `trace_id` and `span_id` from context.

Sub-package `genai` (`github.com/hollis-labs/otel/genai`):

- Attribute key constants for OTel GenAI semantic conventions (`gen_ai.system`, `gen_ai.request.model`, `gen_ai.operation.name`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.response.finish_reason`).
- `ModelCallSpan(ctx, model, operation)` — span named `gen_ai.<operation>` with required attributes.
- `RecordTokenUsage(span, inputTokens, outputTokens)` — sets token usage attributes.
- `RecordModelLatency(ctx, model, duration)` — records `gen_ai.client.operation.duration` histogram.

Sub-package `propagation` (`github.com/hollis-labs/otel/propagation`):

- `HTTPMiddleware(next http.Handler) http.Handler` — server middleware that extracts `traceparent`, starts a server span, and records HTTP attributes/status.
- `InjectHTTP(ctx, req)` — injects W3C trace context into outgoing HTTP request headers.
- `ExtractMCP(params)` / `InjectMCP(ctx, params)` — custom MCP propagation via `_traceparent`/`_tracestate` keys in tool call params.

Sub-package `redaction` (`github.com/hollis-labs/otel/redaction`):

- `Denylist() []string` — default attribute keys that should be removed by downstream exporters or wrappers.
- `ShouldRedact(key string) bool` — true for denylisted keys when `FE_OTEL_REDACT_PROMPTS` is not `false`.
- `SpanProcessor() sdktrace.SpanProcessor` — compatibility shim that preserves the denylist decision but does not mutate `sdktrace.ReadOnlySpan`.

## Architecture Notes

The `Init` function builds a `Resource` via the `internal` package that merges `resource.Default()` with schemaless attributes for `service.name`, `service.version`, `deployment.environment`, and a fixed `service.namespace="fragments-engine"`. It installs a batching `TracerProvider` with an OTLP HTTP exporter (insecure) and a composite `TraceContext` + `Baggage` text-map propagator.

The `redaction` package is intentionally a denylist helper rather than a mutating span processor. `ReadOnlySpan` cannot be rewritten in `OnEnd`, so export-time enforcement belongs in a wrapper around your exporter. See `docs/observability-contract.md` for the full Fragments Engine observability contract (resource identity, span taxonomy, GenAI conventions, propagation, redaction defaults, sampling, environment variables). That document still reflects some legacy `tiamat*` naming, so treat the code and this README as the current source of truth for `feotel` / `FE_OTEL_*`.

## Dependencies

External (from `go.mod`):

- `go.opentelemetry.io/otel` v1.41.0
- `go.opentelemetry.io/otel/sdk` v1.41.0
- `go.opentelemetry.io/otel/trace` v1.41.0
- `go.opentelemetry.io/otel/metric` v1.41.0
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` v1.41.0
- `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` v1.41.0 (used by the example)

Framework-internal: none.

## Testing

```bash
go test ./...
```

The module now has focused tests for propagation, logging, redaction, and initialization. The runnable example can be exercised with:

```bash
go run ./examples/hello
```

## License

MIT — see `LICENSE` (Copyright 2026 Hollis Labs).
