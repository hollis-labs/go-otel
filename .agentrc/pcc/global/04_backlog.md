---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Backlog

## What is done

- Core `Init()` with OTLP HTTP exporter, env var config, and option overrides
- Domain span helpers: `StartSpan`, `AgentStepSpan`, `ToolCallSpan`, `MemoryReadSpan`, `MemoryWriteSpan`
- GenAI semconv helpers: `ModelCallSpan`, `RecordTokenUsage`, `RecordModelLatency`
- GenAI attribute key constants (system, model, operation, tokens, finish reason)
- HTTP middleware with W3C trace context extraction and server span creation
- HTTP injection helper (`InjectHTTP`)
- MCP propagation: `InjectMCP` / `ExtractMCP` via `_traceparent` key
- Redaction span processor with denylist for prompt/completion content
- Log correlation handler (`NewLogHandler`) injecting trace_id/span_id into slog
- Standard metrics registration (request count, latency, error count)
- OTel Resource builder with Fragments Engine identity attributes
- Observability contract document (v1.0)
- All 6 Fragments Engine projects instrumented as consumers

## Inferred next priorities

1. **Unit tests** — No test files exist. Add tests for Init, span helpers, propagation, redaction, and GenAI helpers.
2. **Extended redaction denylist** — The contract specifies `*.api_key`, `*.token`, `*.secret`, `*.password` patterns, but the redaction processor only handles `gen_ai.content.prompt` and `gen_ai.content.completion`. Implement wildcard/suffix matching.
3. **GenAI token counter metric** — The contract defines `gen_ai.client.token.usage` as a Counter metric, but only `RecordModelLatency` (histogram) is implemented. Add `RecordTokenUsageMetric()` that increments the counter.
4. **Meter provider setup in Init()** — `Init()` only configures the trace provider. Add meter provider initialization with OTLP metric exporter so `RegisterMetrics()` and GenAI metrics export correctly.
5. **DB query span helper** — The contract defines `{app}.db.query` spans with `db.operation` and `db.statement` (redacted) attributes. No helper exists yet.
6. **Structured error recording** — Add a helper that calls `span.RecordError(err)` and `span.SetStatus(codes.Error, msg)` in one call.
7. **Release/tagging process** — Establish semver tagging for eventual module proxy publishing.
