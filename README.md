# go-otel

[![Go Reference](https://pkg.go.dev/badge/github.com/hollis-labs/go-otel.svg)](https://pkg.go.dev/github.com/hollis-labs/go-otel)

`go-otel` is an opinionated OpenTelemetry bootstrap for Go services. It
wires up an OTLP HTTP trace exporter and (opt-in) OTLP exporters for
metrics and logs, installs W3C trace context + Baggage propagators, and
provides small helpers for span taxonomy, GenAI semantic conventions, the
`hollis.*` metric instrument set, an opinionated `Recorder` layer over
those instruments, HTTP and MCP-style propagation, slog trace correlation
(with optional OTLP log fan-out), Go-runtime metrics, resource detectors,
signal-aware shutdown, and a denylist for sensitive prompt/completion
attributes.

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

- `Init(ctx, opts...) (shutdown, err)` — installs an OTLP HTTP TracerProvider and W3C+Baggage propagators. With `WithMetricsEnabled`, also installs an OTLP HTTP MeterProvider behind a PeriodicReader. With `WithLogsEnabled`, also installs an OTLP HTTP LoggerProvider behind a BatchProcessor on the global logger provider. With `WithRuntimeMetrics`, starts the upstream Go-runtime instrumentation against the installed MeterProvider.
- Options: `WithServiceName`, `WithServiceVersion`, `WithServiceNamespace`, `WithEnvironment`, `WithOTLPEndpoint` (default `localhost:4318`), `WithSampler`, `WithMetricsEnabled` (default OFF), `WithRuntimeMetrics` (default OFF; requires `WithMetricsEnabled`), `WithLogsEnabled` (default OFF), `WithResourceDetectors(...resource.Option)` (default empty).
- `DefaultDetectors() []resource.Option` — baseline detectors (host, OS, process, container) that don't make network calls. Pair with `WithResourceDetectors(DefaultDetectors()...)` for sensible default resource attribution.
- `NotifyShutdown() (ctx, stop)` — convenience wrapper around `signal.NotifyContext` bound to SIGTERM, SIGINT, and os.Interrupt.
- `ShutdownWithTimeout(shutdown, timeout)` — calls shutdown with a fresh `context.Background()` bounded by the given timeout. Suitable for use inside `defer`.
- `InitOrWarn(ctx, logf, timeout, opts...) (shutdown func())` — `Init` with the surrounding glue: a failure is reported through `logf` (matches `log.Printf`) and yields a no-op shutdown, and a successful shutdown is bound to `ShutdownWithTimeout(_, timeout)` so a hung exporter cannot hang process exit. Does not decide whether telemetry is enabled.
- `EnvironmentFromEnv(appEnvVar, fallback)` — deployment-environment tag for `WithEnvironment`: `HOLLIS_ENV`, then `appEnvVar` (`""` skips it), then `fallback`; values trimmed, blank counts as unset.
- `EnabledFromEnv(appEnvVar) bool` — opt-in telemetry gate with presence-based precedence: if `HOLLIS_OTEL_ENABLED` is set non-blank (trimmed) it decides alone (`1`/`true`/`yes`/`on`, case-insensitive, enables; anything else disables) and `appEnvVar` is ignored; only when it is unset or blank is `appEnvVar` (`""` skips it) checked. No fallback: disabled is the floor. `Init`/`InitOrWarn` do not call it; see [Opt-in gate](#opt-in-gate-enabledfromenv).
- `StartSpan(ctx, name, opts...)` — wraps the global tracer.
- `AgentStepSpan(ctx, step)` — `hollis.agent.step` span with `hollis.agent.step.name` attribute.
- `ToolCallSpan(ctx, tool)` — `hollis.tool.call` span with `hollis.tool.name` attribute.
- `MemoryReadSpan(ctx, namespace, key)` / `MemoryWriteSpan(ctx, namespace, key)` — `hollis.memory.read` / `hollis.memory.write` spans.
- `RegisterMetrics(meter) (*Metrics, error)` — registers the `hollis.*` instrument set (HTTP request count/duration, agent turn duration, tool call count/duration, message count/duration, SSE active connections / reconnects, queue depth, provider input/output tokens, context-window token-budget usage). Cardinality discipline: labels are bounded (`app`, `route`, `status_code`, `provider`, `model`, `kind`, `result`, `tool_name`, `stream_type`, `queue_name`, `runtime_kind`); session/task/agent/message IDs are trace-only and must not be attached.
- `RegisterRecorder(meter, app) (*Recorder, error)` / `NewRecorder(metrics, app)` — wraps `*Metrics` with a bound `app` label and exposes typed helpers per instrument family so call sites don't re-implement label discipline (which labels go on count vs duration, the +1/-1 SSE connection-gauge dance, the signed `QueueDepth` delta, the shared input/output token-counter labels). Methods: `HTTPRequest`, `AgentTurn`, `ToolCall`, `Message`, `ProviderTokens`, `ContextTokenBudget`, `SSEConnectionOpened` / `SSEConnectionClosed` / `SSEReconnect`, `QueueDepth`. Use `Recorder.Metrics()` for direct instrument access when you need a label shape the recorder doesn't cover.
- `NewLogHandler(inner slog.Handler) slog.Handler` — wraps an `slog.Handler` to inject `trace_id` and `span_id` from context. Stderr-only.
- `NewSlogHandler(scopeName, stderrInner) slog.Handler` — fan-out handler that wraps `stderrInner` with `NewLogHandler` AND emits to the OTel slog bridge bound to `scopeName`. When `Init` was called with `WithLogsEnabled`, the OTLP side flows through the installed LoggerProvider; otherwise it falls through to the no-op global provider and discards records. Use this when you want stderr logs AND OTLP log export from the same `slog.Logger`.

#### Opt-in gate (`EnabledFromEnv`)

Telemetry is opt-in across hollis-labs apps. `EnabledFromEnv` is the one place that policy lives; the app supplies only its own override variable name and keeps the gate around `InitOrWarn`:

```go
if hotel.EnabledFromEnv("MYAPP_OTEL_ENABLED") {
	shutdown := hotel.InitOrWarn(ctx, log.Printf, 5*time.Second,
		hotel.WithServiceName("myapp"),
		hotel.WithEnvironment(hotel.EnvironmentFromEnv("MYAPP_ENV", "development")),
	)
	defer shutdown()
}
```

```bash
HOLLIS_OTEL_ENABLED=1 myapp                          # on
MYAPP_OTEL_ENABLED=yes myapp                         # on (global unset)
HOLLIS_OTEL_ENABLED=false MYAPP_OTEL_ENABLED=yes myapp  # OFF: a set global decides alone
HOLLIS_OTEL_ENABLED=banana MYAPP_OTEL_ENABLED=yes myapp # OFF: garbage global still decides
HOLLIS_OTEL_ENABLED=" " MYAPP_OTEL_ENABLED=yes myapp    # on: blank global falls through
myapp                                                # off (also off for "", "0", garbage)
```

Findings from reading (not running) the apps' source when this was added: Loom already gates opt-in (bare `OTEL_ENABLED` / `LOOM_OTEL_ENABLED`); Tether, Nanite and Torque gate opt-out (`*_OTEL_DISABLED`), so adopting this flips their default and needs owner sign-off; **Nil, Hadron and Tesseract have no telemetry gate today** and call `Init` unconditionally.

#### Recorder example

```go
rec, err := hotel.RegisterRecorder(otel.Meter("my-app"), "my-app")
if err != nil {
    log.Fatal(err)
}

// HTTP middleware path:
rec.HTTPRequest(ctx, route, statusCode, time.Since(start))

// Tool call from an agent loop:
rec.ToolCall(ctx, "memory_write", "ok", time.Since(start))

// Token accounting after a model call:
rec.ProviderTokens(ctx, "anthropic", "claude-opus-4-7", inputTokens, outputTokens)
rec.ContextTokenBudget(ctx, "anthropic", "claude-opus-4-7", contextUsed)

// SSE connection lifecycle:
rec.SSEConnectionOpened(ctx, "agent_events")
defer rec.SSEConnectionClosed(ctx, "agent_events")
```

### Sub-package `genai`

OpenTelemetry GenAI semantic-convention helpers.

- Attribute key constants for `gen_ai.system`, `gen_ai.request.model`, `gen_ai.operation.name`, `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`, `gen_ai.response.finish_reason`.
- `ModelCallSpan(ctx, model, operation)` — span named `gen_ai.<operation>` with required attributes.
- `RecordTokenUsage(span, inputTokens, outputTokens)` — sets token usage attributes.
- `RecordModelLatency(ctx, model, duration)` — records the `gen_ai.client.operation.duration` histogram.

### Sub-package `propagation`

- `HTTPMiddleware(next http.Handler, opts ...MiddlewareOption) http.Handler` — server middleware that extracts `traceparent`, starts a server span, and records HTTP attributes/status. With `WithMetricRecorder(rec)`, also emits `hollis.http.request.count` / `.duration` per request via `rec.HTTPRequest`. With `WithRouteResolver(fn)`, uses `fn(r)` to compute the bounded-cardinality `route` label (default: `r.URL.Path`, cardinality-unsafe for production).
- `HTTPMetricRecorder` interface (`HTTPRequest(ctx, route, statusCode, duration)`) — satisfied by `*hotel.Recorder`; defined in the propagation package so it can be implemented without importing `hotel`.
- `InjectHTTP(ctx, req)` — injects W3C trace context into outgoing HTTP request headers.
- `ExtractMCP(ctx, params)` / `InjectMCP(ctx, params)` — propagation through `_traceparent` / `_tracestate` keys in an MCP-style tool-call params map. **Deprecated** in favor of the `_meta` pair below.
- `InjectMCPMeta(ctx, meta map[string]any) map[string]any` / `ExtractMCPMeta(ctx, meta map[string]any) context.Context` — the same W3C trace context carried in an MCP request's `_meta` object under the bare keys `_traceparent` / `_tracestate`, the protocol-correct location. `InjectMCPMeta` returns a copy (the caller's map is not modified) and keeps unrelated `_meta` keys; `ExtractMCPMeta` preserves the caller's context. `_meta` only: neither reads or writes tool `arguments`. Baggage is not carried. The go-sdk's `mcp.Meta` is a `map[string]any`, so convert with `map[string]any(params.Meta)` and back:

  ```go
  params.Meta = mcp.Meta(propagation.InjectMCPMeta(ctx, params.Meta))   // client
  ctx = propagation.ExtractMCPMeta(ctx, req.Params.Meta)                 // server
  ```

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
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP exporter endpoint (serves `/v1/traces` always, plus `/v1/metrics` when `WithMetricsEnabled` and `/v1/logs` when `WithLogsEnabled`) | `localhost:4318` |
| `OTEL_METRIC_EXPORT_INTERVAL` | PeriodicReader interval for the metric exporter (read by the SDK; only meaningful when `WithMetricsEnabled`) | `15s` |
| `OTEL_BLRP_SCHEDULE_DELAY` / `OTEL_BLRP_EXPORT_TIMEOUT` / `OTEL_BLRP_MAX_QUEUE_SIZE` / `OTEL_BLRP_MAX_EXPORT_BATCH_SIZE` | BatchProcessor tuning for the log exporter (read by the SDK; only meaningful when `WithLogsEnabled`) | SDK defaults |
| `HOLLIS_OTEL_ENABLED` | read by `EnabledFromEnv` only (never by `Init`): when set non-blank it decides the gate alone (`1`/`true`/`yes`/`on` on, anything else off), acting as a portfolio-wide kill switch that overrides app vars; blank/unset defers to the app var | unset (disabled) |
| `HOLLIS_OTEL_REDACT_PROMPTS` | when not `false`, `redaction.ShouldRedact` returns true for denylisted GenAI content keys | unset (treated as enabled) |

Options passed to `Init` always take precedence over environment variables.

## Testing

```bash
go test ./...
go test -race ./...
```

## License

MIT — see [`LICENSE`](./LICENSE) (Copyright Hollis Labs).
