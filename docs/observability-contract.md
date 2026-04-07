# Fragments Engine Observability Contract v1.0

> Effective: 2026-03-05
> Scope: All Fragments Engine applications (Volon, Cortex, Hadron, Nanite, Carrier, Mentat)

This contract defines the observability standards for the Fragments Engine suite. All instrumented applications MUST comply with these requirements.

---

## 1. Resource Identity (Required)

Every telemetry signal (trace, metric, log) MUST include these resource attributes:

| Attribute | Required | Example | Notes |
|-----------|----------|---------|-------|
| `service.name` | Yes | `volon`, `cortex`, `hadron` | Unique per application |
| `service.version` | Yes | `0.5.0`, `1.0.0` | Semantic version |
| `service.namespace` | Yes | `tiamat` | Always `tiamat` |
| `deployment.environment` | Yes | `development`, `production` | From `OTEL_RESOURCE_ATTRIBUTES` or option |

Set via `tiamatotel.Init()` options or environment variables.

---

## 2. Span Taxonomy

### 2.1 Root Spans (one per entry point)

| Entry Point | Span Name Pattern | Required Attributes |
|-------------|-------------------|---------------------|
| HTTP request | `HTTP {method} {route}` | `http.method`, `http.route`, `http.status_code` |
| CLI invocation | `{app}.cli.{command}` | `cli.command`, `cli.args` (redacted) |
| Scheduled job | `{app}.scheduler.tick` | `scheduler.task_id` |
| Chat session | `volon.chat.session` | `chat.session_id`, `chat.model` |
| Blueprint run | `hadron.blueprint.run` | `hadron.blueprint`, `hadron.run_id` |

### 2.2 Child Span Types

| Span Name | Used By | Required Attributes |
|-----------|---------|---------------------|
| `tiamat.agent.step` | Volon | `agent.step.name`, `agent.step.index` |
| `tiamat.tool.call` | Volon, Hadron | `tool.name`, `tool.status` |
| `tiamat.model.call` | Volon | GenAI attributes (see Section 3) |
| `tiamat.memory.read` | Cortex | `cortex.namespace`, `cortex.key` |
| `tiamat.memory.write` | Cortex | `cortex.namespace`, `cortex.key`, `cortex.revision` |
| `{app}.db.query` | Any | `db.operation`, `db.statement` (redacted) |

### 2.3 Span Status

- Set `OK` on success
- Set `ERROR` with description on failure
- Record exceptions via `span.RecordError(err)`

---

## 3. GenAI Semantic Conventions

Applies to: **Volon** (LLM chat, agent executor). Other apps: not applicable unless they add LLM calls.

Follow [OTel GenAI semconv](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/).

### 3.1 Required Span Attributes

| Attribute | Type | Example |
|-----------|------|---------|
| `gen_ai.system` | string | `anthropic`, `openai` |
| `gen_ai.request.model` | string | `claude-sonnet-4-20250514`, `gpt-4o` |
| `gen_ai.operation.name` | string | `chat`, `completion` |

### 3.2 Required Span Events / Post-Completion Attributes

| Attribute | Type | Notes |
|-----------|------|-------|
| `gen_ai.usage.input_tokens` | int | Set after response received |
| `gen_ai.usage.output_tokens` | int | Set after response received |
| `gen_ai.response.finish_reason` | string | `stop`, `max_tokens`, `tool_use` |

### 3.3 Prohibited by Default

| Attribute | Default | Opt-in |
|-----------|---------|--------|
| `gen_ai.content.prompt` | REDACTED | `TIAMAT_OTEL_REDACT_PROMPTS=false` |
| `gen_ai.content.completion` | REDACTED | `TIAMAT_OTEL_REDACT_PROMPTS=false` |

### 3.4 GenAI Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `gen_ai.client.token.usage` | Counter | `gen_ai.system`, `gen_ai.request.model`, `token.type` (input/output) |
| `gen_ai.client.operation.duration` | Histogram | `gen_ai.system`, `gen_ai.request.model`, `gen_ai.operation.name` |

---

## 4. Logging

### 4.1 Log Correlation (Required)

All structured logs MUST include trace context when available:

| Field | Source |
|-------|--------|
| `trace_id` | From active span context |
| `span_id` | From active span context |

Use `tiamatotel.NewLogHandler()` to wrap `slog.Handler` for automatic injection.

### 4.2 Log Levels

Standard `slog` levels. No additional requirements beyond correlation.

---

## 5. Context Propagation

### 5.1 HTTP (Required)

W3C Trace Context headers:
- `traceparent` — injected/extracted automatically via OTel HTTP middleware
- `tracestate` — preserved if present

All outgoing HTTP calls to other Fragments Engine services MUST propagate trace context. Use `otelhttp.Transport` or `propagation.InjectHTTP()`.

### 5.2 MCP Tool Calls

Custom propagation via tool call parameters:
- Inject: `propagation.InjectMCP(ctx, params)` — adds `_traceparent` key
- Extract: `propagation.ExtractMCP(params)` — reads `_traceparent` key

### 5.3 Message Queues / Async

If async messaging is added in the future, propagate trace context via message headers following W3C conventions.

---

## 6. Redaction & Sampling

### 6.1 Redaction Defaults

| Setting | Default | Env Var |
|---------|---------|---------|
| Prompt/response content | Redacted | `TIAMAT_OTEL_REDACT_PROMPTS=false` to disable |
| SQL statements | Redacted to operation only | — |
| API keys / tokens | Always redacted | — |
| User PII | Always redacted | — |

### 6.2 Attribute Denylist

The redaction processor strips these attributes by default:
- `gen_ai.content.prompt`
- `gen_ai.content.completion`
- `db.statement` (truncated to first 100 chars)
- Any attribute matching `*.api_key`, `*.token`, `*.secret`, `*.password`

### 6.3 Sampling

| Environment | Default Sampler |
|-------------|-----------------|
| `development` | `AlwaysSample` |
| `production` | `TraceIDRatioBased(0.1)` |

Override via `OTEL_TRACES_SAMPLER` and `OTEL_TRACES_SAMPLER_ARG` (standard OTel env vars).

---

## 7. Configuration Contract

### 7.1 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | (required) | Application name |
| `OTEL_SERVICE_VERSION` | `unknown` | Application version |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | `grpc` or `http/protobuf` |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace=tiamat` | Additional resource attrs (comma-separated k=v) |
| `OTEL_TRACES_SAMPLER` | `always_on` | OTel sampler name |
| `OTEL_TRACES_SAMPLER_ARG` | — | Sampler argument (e.g., ratio) |
| `TIAMAT_OTEL_REDACT_PROMPTS` | `true` | Redact prompt/response content |
| `TIAMAT_OTEL_LOG_CORRELATION` | `true` | Inject trace_id/span_id into logs |

### 7.2 Programmatic Override

All env vars can be overridden via `tiamatotel.Init()` options. Programmatic values take precedence.

### 7.3 Graceful Degradation

If OTLP endpoint is unreachable:
- Traces/metrics are dropped silently (no error propagation to app)
- A warning is logged once at startup
- Application continues to function normally

---

## 8. Per-App Applicability Matrix

| Requirement | Volon | Cortex | Hadron | Nanite | Carrier | Mentat |
|-------------|-------|--------|--------|--------|---------|--------|
| Resource identity | Yes | Yes | Yes | Yes | Yes | Yes |
| HTTP middleware | Yes | Yes | Yes | No | No | No |
| CLI root spans | Yes | Yes | Yes | Yes | No | No |
| GenAI spans | Yes | No | No | No | No | No |
| Memory spans | No | Yes | No | No | No | No |
| Tool call spans | Yes | No | Yes | No | No | No |
| Agent step spans | Yes | No | No | No | No | No |
| Blueprint run spans | No | No | Yes | No | No | No |
| Log correlation | Yes | Yes | Yes | Yes | Yes | No |
| Cross-svc propagation | Yes | Yes | Yes (recv) | No | No | No |
| Hook-based emission | No | No | No | No | No | Yes |

---

## 9. Instrument New App Checklist

When adding OTel to a new Fragments Engine app:

1. Add `github.com/hollis-labs/otel` to `go.mod` (or `opentelemetry-python` for Python)
2. Call `tiamatotel.Init()` in `main()` with `WithServiceName()`
3. Defer `shutdown(ctx)` for graceful cleanup
4. Add HTTP middleware if the app has an HTTP server
5. Add domain-specific spans (check applicability matrix above)
6. Wrap `slog.Default()` with `tiamatotel.NewLogHandler()` for log correlation
7. Verify with local Jaeger: `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run .`
8. Add `OTEL_SERVICE_NAME` to the app's `.env.local` or startup docs
