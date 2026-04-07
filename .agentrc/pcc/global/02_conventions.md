---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Conventions

## Span naming

All domain spans use a dotted namespace prefix:

| Pattern | Example | Used by |
|---------|---------|---------|
| `tiamat.agent.step` | Agent step span | Volon |
| `tiamat.tool.call` | Tool invocation span | Volon, Hadron |
| `tiamat.memory.read` | Context read span | Cortex |
| `tiamat.memory.write` | Context write span | Cortex |
| `gen_ai.{operation}` | `gen_ai.chat` | Volon (GenAI) |
| `HTTP {method} {path}` | `GET /api/v1/tasks` | HTTP middleware |

## Attribute naming

- Tiamat-specific attributes use `tiamat.` prefix (e.g., `tiamat.agent.step.name`, `tiamat.tool.name`, `tiamat.memory.namespace`)
- GenAI attributes follow OTel semconv exactly: `gen_ai.system`, `gen_ai.request.model`, `gen_ai.usage.input_tokens`, etc.
- HTTP attributes: `http.method`, `http.target`, `http.status_code`
- Resource attributes: `service.name`, `service.version`, `service.namespace` (always `tiamat`), `deployment.environment`

## Environment variable contract

| Variable | Default | Purpose |
|----------|---------|---------|
| `OTEL_SERVICE_NAME` | (required) | Application identity |
| `OTEL_SERVICE_VERSION` | `unknown` | Application version |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4318` | OTLP HTTP endpoint |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace=tiamat` | Additional resource attrs |
| `OTEL_TRACES_SAMPLER` | `always_on` | Sampler selection |
| `TIAMAT_OTEL_REDACT_PROMPTS` | `true` | Redact GenAI prompt/completion content |
| `TIAMAT_OTEL_LOG_CORRELATION` | `true` | Inject trace_id/span_id into slog |

## Adding new instrumentation

1. **New span helper**: Add a function in `tracing.go` (or a sub-package if domain-specific). Follow the pattern: accept `context.Context`, return `(context.Context, trace.Span)`, set attributes via `trace.WithAttributes()`.
2. **New attribute keys**: Define as `attribute.Key` constants in the relevant package (root or `genai/`).
3. **New metric**: Add to `TiamatMetrics` struct in `metrics.go` and register in `RegisterMetrics()`.
4. **New sub-package**: Create a directory, add a `doc.go` or attributes file, follow existing patterns in `genai/` or `propagation/`.
5. **Update the contract**: Any new span types or required attributes must be documented in `docs/observability-contract.md`.

## Redaction rules

- `gen_ai.content.prompt` and `gen_ai.content.completion` are redacted by default.
- Controlled by `TIAMAT_OTEL_REDACT_PROMPTS=false` to disable.
- Additional denylist patterns (`*.api_key`, `*.token`, `*.secret`, `*.password`) are specified in the contract but not yet implemented in the processor.
