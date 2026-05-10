# go-otel

[![Go Reference](https://pkg.go.dev/badge/github.com/hollis-labs/go-otel.svg)](https://pkg.go.dev/github.com/hollis-labs/go-otel)

`go-otel` is an opinionated OpenTelemetry bootstrap for Go services. It wires
up an OTLP HTTP exporter, installs W3C trace context + Baggage propagators,
and provides small helpers for span taxonomy, GenAI semantic conventions,
HTTP and MCP-style propagation, slog trace correlation, and a denylist for
sensitive prompt/completion attributes.

The package name is `feotel`.

## Status

Pre-1.0 (v0.x). The public API is exercised by tests and a runnable example
but minor breaks are possible across pre-1.0 minor versions. See
[`CHANGELOG.md`](./CHANGELOG.md).

## Install

```bash
go get github.com/hollis-labs/go-otel
```

Requires Go 1.25 or later.

## Quickstart

```go
package main

import (
    "context"
    "log"

    feotel "github.com/hollis-labs/go-otel"
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

A runnable example using the stdout exporter (no collector required) lives at
[`examples/hello/main.go`](./examples/hello/main.go) and demonstrates the
core span helpers and the `genai` sub-package:

```bash
go run ./examples/hello
```

## Documentation

API reference: <https://pkg.go.dev/github.com/hollis-labs/go-otel>

### Top-level package `feotel`

- `Init(ctx, opts...) (shutdown, err)` — installs an OTLP HTTP TracerProvider and W3C+Baggage propagators.
- Options: `WithServiceName`, `WithServiceVersion`, `WithServiceNamespace`, `WithEnvironment`, `WithOTLPEndpoint` (default `localhost:4318`), `WithSampler`.
- `StartSpan(ctx, name, opts...)` — wraps the global tracer.
- `AgentStepSpan(ctx, step)` — `fe.agent.step` span with `fe.agent.step.name` attribute.
- `ToolCallSpan(ctx, tool)` — `fe.tool.call` span with `fe.tool.name` attribute.
- `MemoryReadSpan(ctx, namespace, key)` / `MemoryWriteSpan(ctx, namespace, key)` — `fe.memory.read` / `fe.memory.write` spans.
- `RegisterMetrics(meter) (*Metrics, error)` — registers `fe.request.count`, `fe.request.latency`, `fe.error.count` instruments.
- `NewLogHandler(inner slog.Handler) slog.Handler` — wraps an `slog.Handler` to inject `trace_id` and `span_id` from context.

### Sub-package `genai`

OpenTelemetry GenAI semantic-convention helpers.

- Attribute key constants for `gen_ai.system`, `gen_ai.request.model`, `gen_ai.operation.name`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.response.finish_reason`.
- `ModelCallSpan(ctx, model, operation)` — span named `gen_ai.<operation>` with required attributes.
- `RecordTokenUsage(span, inputTokens, outputTokens)` — sets token usage attributes.
- `RecordModelLatency(ctx, model, duration)` — records the `gen_ai.client.operation.duration` histogram.

### Sub-package `propagation`

- `HTTPMiddleware(next http.Handler) http.Handler` — server middleware that extracts `traceparent`, starts a server span, and records HTTP attributes/status.
- `InjectHTTP(ctx, req)` — injects W3C trace context into outgoing HTTP request headers.
- `ExtractMCP(params)` / `InjectMCP(ctx, params)` — propagation through `_traceparent` / `_tracestate` keys in an MCP-style tool-call params map.

### Sub-package `redaction`

- `Denylist() []string` — default attribute keys that should be removed by downstream exporters or wrappers.
- `ShouldRedact(key string) bool` — true for denylisted keys when `FE_OTEL_REDACT_PROMPTS` is not set to `false`.
- `SpanProcessor() sdktrace.SpanProcessor` — compatibility shim that preserves the denylist decision but does not mutate `sdktrace.ReadOnlySpan` (which is immutable). Real enforcement belongs in a wrapping exporter.

## Conventions

This library promotes an `fe.*` attribute / span / metric naming convention
(historical, retained for stability across pre-1.0 minor versions). Sub-package
`genai` uses standard OTel `gen_ai.*` semconv. You are free to ignore the
`fe.*` helpers and use this library purely for its `Init` / propagation /
slog-handler / redaction surfaces with your own attribute schema.

## Environment variables

| Variable | Effect | Default |
| --- | --- | --- |
| `OTEL_SERVICE_NAME` | service.name resource attribute | `""` |
| `OTEL_SERVICE_VERSION` | service.version resource attribute | `"unknown"` |
| `OTEL_SERVICE_NAMESPACE` | service.namespace resource attribute (omitted when empty) | `""` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP exporter endpoint | `localhost:4318` |
| `FE_OTEL_REDACT_PROMPTS` | when not `false`, `redaction.ShouldRedact` returns true for denylisted GenAI content keys | unset (treated as enabled) |

Options passed to `Init` always take precedence over environment variables.

## Testing

```bash
go test ./...
go test -race ./...
```

## License

MIT — see [`LICENSE`](./LICENSE) (Copyright Hollis Labs).
