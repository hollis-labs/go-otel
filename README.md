# go-otel

[![Go Reference](https://pkg.go.dev/badge/github.com/hollis-labs/go-otel.svg)](https://pkg.go.dev/github.com/hollis-labs/go-otel)

`go-otel` is an opinionated OpenTelemetry bootstrap for Go services. It wires
up an OTLP HTTP trace exporter and (opt-in) metric exporter, installs W3C
trace context + Baggage propagators, and provides small helpers for span
taxonomy, GenAI semantic conventions, the `hollis.*` metric instrument set,
HTTP and MCP-style propagation, slog trace correlation, and a denylist for
sensitive prompt/completion attributes.

The Go package name is `hotel` ("Hollis OTel"). The library promotes a
`hollis.*` taxonomy for spans, attributes, and metrics emitted on the wire,
while keeping the Go-side identifier short and provider-neutral.

## Status

Pre-1.0 (v0.x). The public API is exercised by tests and a runnable example
but minor breaks are possible across pre-1.0 minor versions. See
[`CHANGELOG.md`](./CHANGELOG.md).

## Install

```bash
go get github.com/hollis-labs/go-otel
```

Requires Go 1.26 or later (see [`go.mod`](./go.mod)).

## Quickstart

```go
package main

import (
    "context"
    "log"

    "github.com/hollis-labs/go-otel"
)

func main() {
    ctx := context.Background()

    shutdown, err := hotel.Init(ctx,
        hotel.WithServiceName("my-service"),
        hotel.WithServiceVersion("0.1.0"),
        hotel.WithEnvironment("development"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

    ctx, span := hotel.StartSpan(ctx, "my.operation")
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

### Top-level package `hotel`

- `Init(ctx, opts...) (shutdown, err)` — installs an OTLP HTTP TracerProvider and W3C+Baggage propagators. With `WithMetricsEnabled`, also installs an OTLP HTTP MeterProvider behind a PeriodicReader.
- Options: `WithServiceName`, `WithServiceVersion`, `WithServiceNamespace`, `WithEnvironment`, `WithOTLPEndpoint` (default `localhost:4318`), `WithSampler`, `WithMetricsEnabled` (default OFF).
- `StartSpan(ctx, name, opts...)` — wraps the global tracer.
- `AgentStepSpan(ctx, step)` — `hollis.agent.step` span with `hollis.agent.step.name` attribute.
- `ToolCallSpan(ctx, tool)` — `hollis.tool.call` span with `hollis.tool.name` attribute.
- `MemoryReadSpan(ctx, namespace, key)` / `MemoryWriteSpan(ctx, namespace, key)` — `hollis.memory.read` / `hollis.memory.write` spans.
- `RegisterMetrics(meter) (*Metrics, error)` — registers the `hollis.*` instrument set (HTTP request count/duration, agent turn duration, tool call count/duration, message count/duration, SSE active connections / reconnects, queue depth, provider input/output tokens, context-window token-budget usage). Cardinality discipline: labels are bounded (`app`, `route`, `status_code`, `provider`, `model`, `kind`, `result`, `tool_name`, `stream_type`, `queue_name`, `runtime_kind`); session/task/agent/message IDs are trace-only and must not be attached.
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
- `ShouldRedact(key string) bool` — true for denylisted keys when `HOLLIS_OTEL_REDACT_PROMPTS` is not set to `false`.
- `SpanProcessor() sdktrace.SpanProcessor` — compatibility shim that preserves the denylist decision but does not mutate `sdktrace.ReadOnlySpan` (which is immutable). Real enforcement belongs in a wrapping exporter.

## Conventions

This library promotes a `hollis.*` attribute / span / metric naming convention
for the helpers it exposes. Sub-package `genai` uses standard OTel `gen_ai.*`
semconv. You are free to ignore the `hollis.*` helpers and use this library
purely for its `Init` / propagation / slog-handler / redaction surfaces with
your own attribute schema.

## Environment variables

| Variable | Effect | Default |
| --- | --- | --- |
| `OTEL_SERVICE_NAME` | service.name resource attribute | `""` |
| `OTEL_SERVICE_VERSION` | service.version resource attribute | `"unknown"` |
| `OTEL_SERVICE_NAMESPACE` | service.namespace resource attribute (omitted when empty) | `""` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP exporter endpoint (used for both `/v1/traces` and, when metrics are enabled, `/v1/metrics`) | `localhost:4318` |
| `OTEL_METRIC_EXPORT_INTERVAL` | PeriodicReader interval for the metric exporter (read by the SDK; only meaningful when `WithMetricsEnabled`) | `15s` |
| `HOLLIS_OTEL_REDACT_PROMPTS` | when not `false`, `redaction.ShouldRedact` returns true for denylisted GenAI content keys | unset (treated as enabled) |

Options passed to `Init` always take precedence over environment variables.

## Testing

```bash
go test ./...
go test -race ./...
```

## License

MIT — see [`LICENSE`](./LICENSE) (Copyright Hollis Labs).
