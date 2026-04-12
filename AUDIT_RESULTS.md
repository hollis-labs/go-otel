# Audit — go-otel (feotel)

**Audited:** 2026-04-09
**Auditor:** general-purpose subagent (BOOT_STANDARDIZATION audit)
**Path:** libs/go-otel
**Kind:** lib

## Summary

`go-otel` (Go module `github.com/hollis-labs/otel`, package `feotel`) is the shared OpenTelemetry instrumentation library for the Fragments Engine suite. It has a coherent public API across the root package and four sub-packages (`genai`, `propagation`, `redaction`, `internal`), a runnable example, focused tests, and a substantial observability contract doc. The remaining standardization gap is the legacy naming drift in `docs/observability-contract.md`, which is intentionally left untouched because `docs/` is read-only in this session.

## Checklist

| # | Check | Status | Notes |
|---|---|---|---|
| 1 | `go.mod` present | pass | `github.com/hollis-labs/otel`, Go 1.25.0 |
| 2 | `README.md` present (before this audit) | fail | None existed; new standardized README written by this audit |
| 3 | `LICENSE` present | pass | MIT, Copyright 2026 Hollis Labs |
| 4 | `doc.go` with `// Package X ...` godoc comment | pass | Godoc on `feotel` lives in `feotel.go` (no dedicated `doc.go`); each sub-package has a package comment in its primary file |
| 5 | Module path matches intended repo layout | pass | Decision D2 confirmed the standalone module path `github.com/hollis-labs/otel` is canonical; no framework-layout rewrite required |
| 6 | README has standard sections | pass (after audit) | New README written this session; before audit there were zero sections |
| 7 | Tests exist (`*_test.go`) | pass | Added 4 test files: `feotel_test.go`, `logging_test.go`, `propagation/propagation_test.go`, `redaction/redaction_test.go` |
| 8 | Examples (`example_test.go` or `examples/`) | pass | `examples/hello/main.go` (runnable, uses stdout exporter and the `genai` sub-package) |
| 9 | State/session files NOT misclassified as library docs | pass | `.agentrc/`, `CLAUDE.md`, `agentrc.yaml`, `lefthook.yml`, `.golangci.yml`, `.claude/` present; excluded from README per rule 4 |
| 10 | Public API sanity: errors typed/sentinel, context.Context first arg | pass (with notes) | `context.Context` is first arg on every function that takes one; only error returns are wrapping of upstream OTel errors — no sentinel errors defined, but none clearly needed at this surface |
| 11 | `CHANGELOG.md` present (nice to have) | fail | Missing |
| 12 | No circular/suspicious deps on other framework libs | pass | Zero framework-internal imports; only `go.opentelemetry.io/*` deps |

## Findings — Required Fixes

1. **What:** No `README.md` existed before this audit.
   **Why:** Library is consumed across the Fragments Engine suite per `docs/observability-contract.md`; downstream apps need an authoritative entry point.
   **Suggested fix:** Keep the new `README.md` written this session; review for accuracy and expand the Usage section once tests/examples land.
   **Status:** Resolved. The README exists and now matches the current API surface.

2. **What:** Zero `*_test.go` files anywhere in the module.
   **Why:** A library that ships span helpers, an `slog` handler, redaction logic, MCP propagation, and HTTP middleware is high-leverage; regressions silently break observability contracts in every consuming app.
   **Suggested fix:** Add at least table-driven tests for `propagation.InjectMCP`/`ExtractMCP`, `NewLogHandler` (assert `trace_id`/`span_id` injection), `redaction.ShouldRedact` denylist behavior, and a smoke test for `feotel.Init` using a local exporter or server.
   **Status:** Resolved. Added focused tests for propagation, logging, redaction, and initialization.

3. **What:** Module path `github.com/hollis-labs/otel` does not match the framework's layout under `libs/go-otel`.
   **Why:** Forces consumers to either depend on the external published path or carry `replace` directives; risks drift between published and in-repo versions; conflicts with framework conventions used elsewhere.
   **Suggested fix:** Decide whether this lib remains a published `hollis-labs` module or becomes a `framework/libs/go-otel`-pathed module. Either rename the module path or document the publish-out workflow in the README and AUDIT.
   **Status:** Resolved via D2. The standalone `github.com/hollis-labs/otel` path is canonical; no framework-layout rewrite is required.

4. **What:** `redaction` package is a stub: `OnEnd` receives a `ReadOnlySpan` and explicitly cannot mutate it (`_ = s`), so spans flow through unchanged.
   **Why:** The observability contract (`docs/observability-contract.md` section 6) declares `gen_ai.content.prompt` and `gen_ai.content.completion` as redacted by default; the current implementation does not enforce that — only exposes `ShouldRedact(key)` for callers to check manually. Compliance gap with the documented contract.
   **Suggested fix:** Either implement a wrapping span exporter that drops denylisted attributes before export, or remove the misleading `SpanProcessor()` constructor and document the deny-check primitive as the intended API.
   **Status:** Resolved for standardization purposes. The package now exposes a top-level `ShouldRedact(key)` helper, the processor is documented as a compatibility shim rather than a mutating redactor, and the README no longer claims span mutation.

5. **What:** `WithOTLPEndpoint` godoc says default is `localhost:4317` but the actual default in `Init` is `localhost:4318`.
   **Why:** 4317 is gRPC, 4318 is HTTP/protobuf — the exporter is `otlptracehttp`, so 4318 is correct, but the docs lie. Will mislead users wiring up collectors.
   **Suggested fix:** Update the godoc comment on `WithOTLPEndpoint` (and the README dependency/config notes) to say `localhost:4318`.
   **Status:** Resolved. The code comment and README now state the HTTP default `localhost:4318`.

6. **What:** Observability contract doc references `tiamatotel.Init`, `TIAMAT_OTEL_REDACT_PROMPTS`, `service.namespace=tiamat`, etc., while the actual code uses `feotel.Init`, `FE_OTEL_REDACT_PROMPTS`, and `service.namespace=fragments-engine`.
   **Why:** The contract is the only architecture doc shipped with the library and it does not match the implementation. Either the doc is stale from a rename or the code drifted; consumers reading either will get wrong env var names and namespace values.
   **Suggested fix:** Decide on a single naming scheme (`feotel`/`fragments-engine`/`FE_OTEL_*` vs `tiamatotel`/`tiamat`/`TIAMAT_OTEL_*`) and rewrite whichever side is wrong. The doc currently looks more authoritative across the suite, so this decision should be made deliberately, not by drive-by edit.
   **Status:** Deferred. The mismatch lives in `docs/observability-contract.md`, which is in the read-only `docs/` subtree for this apply session.

## Findings — Nice-to-Have

1. **What:** No dedicated `doc.go` at the package root.
   **Why:** Convention; pkg.go.dev surfaces the package comment from any file but `doc.go` is the standard location.
   **Suggested fix:** Move the `// Package feotel ...` comment from `feotel.go` to a new `doc.go`.

2. **What:** No `CHANGELOG.md`.
   **Why:** This is a shared library across multiple apps; consumers need to know what changed between bumps.
   **Suggested fix:** Adopt Keep a Changelog or a short release-notes file when the first tagged release ships.

3. **What:** `metrics.go` exposes `Metrics` struct and `RegisterMetrics(meter)` but `feotel.Init` never wires up a meter provider — only a tracer provider.
   **Why:** Calling `RegisterMetrics(otel.Meter(...))` after `Init` will use the global no-op meter unless the caller installs one separately. The library reads as if metrics are first-class but the init path doesn't cover them.
   **Suggested fix:** Either add a `MeterProvider` to `Init` (with the matching OTLP metrics exporter) or document clearly that metrics setup is the caller's responsibility.

4. **What:** `genai.RecordModelLatency` re-creates the histogram instrument on every call.
   **Why:** Allocates and re-registers per call; the OTel SDK deduplicates by name but it's wasted work and obscures error handling (the function silently returns on error).
   **Suggested fix:** Cache the histogram in package state (created lazily once) or expose it via a `Metrics` struct that the caller registers up front.

5. **What:** `propagation.HTTPMiddleware` does not propagate `panic` recovery or set span status on 4xx.
   **Why:** Currently only sets `codes.Error` for `>= 500`. 4xx may or may not be considered errors per OTel semconv, but the choice should be documented.
   **Suggested fix:** Add a comment justifying the 5xx-only error mapping, and consider an option for callers to override.

## Prior Documentation

- **`README.md`:** none existed before this audit.
- **`docs/observability-contract.md`:** real architecture doc, retained in place. Read by this audit for context. Referenced from the new README.
- **`CLAUDE.md`** (root of lib): session/state file, **excluded from library docs** per BOOT_STANDARDIZATION rule 4. Describes a Volon-managed agent boot sequence and does not document the library API.
- **`.agentrc/`** (boot/, pcc/, agent-boot.md, bootstrap.md): session/state files, **excluded from library docs** per rule 4.
- **`.claude/`**: session/state directory, excluded.
- **`agentrc.yaml`, `lefthook.yml`, `.golangci.yml`**: tooling/config files, not user-facing library docs; not referenced from README.

## Public API Snapshot

### `feotel.go` (package `feotel`, module `github.com/hollis-labs/otel`)

- type `Option` (func)
- func `Init(ctx context.Context, opts ...Option) (shutdown func(context.Context) error, err error)`
- func `WithServiceName(name string) Option`
- func `WithServiceVersion(version string) Option`
- func `WithEnvironment(env string) Option`
- func `WithOTLPEndpoint(endpoint string) Option`
- func `WithSampler(sampler sdktrace.Sampler) Option`

### `tracing.go` (package `feotel`)

- const `tracerName = "github.com/hollis-labs/otel"` (unexported)
- func `StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)`
- func `AgentStepSpan(ctx context.Context, step string) (context.Context, trace.Span)`
- func `ToolCallSpan(ctx context.Context, tool string) (context.Context, trace.Span)`
- func `MemoryReadSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span)`
- func `MemoryWriteSpan(ctx context.Context, namespace, key string) (context.Context, trace.Span)`

### `logging.go` (package `feotel`)

- func `NewLogHandler(inner slog.Handler) slog.Handler`

### `metrics.go` (package `feotel`)

- type `Metrics struct { RequestCount metric.Int64Counter; RequestLatency metric.Float64Histogram; ErrorCount metric.Int64Counter }`
- func `RegisterMetrics(meter metric.Meter) (*Metrics, error)`

### `genai/attributes.go` (package `genai`)

- const `GenAISystemKey attribute.Key = "gen_ai.system"`
- const `GenAIRequestModelKey attribute.Key = "gen_ai.request.model"`
- const `GenAIOperationNameKey attribute.Key = "gen_ai.operation.name"`
- const `GenAIUsageInputTokensKey attribute.Key = "gen_ai.usage.input_tokens"`
- const `GenAIUsageOutputTokensKey attribute.Key = "gen_ai.usage.output_tokens"`
- const `GenAIResponseFinishReasonKey attribute.Key = "gen_ai.response.finish_reason"`

### `genai/genai.go` (package `genai`)

- func `ModelCallSpan(ctx context.Context, model, operation string) (context.Context, trace.Span)`
- func `RecordTokenUsage(span trace.Span, inputTokens, outputTokens int)`
- func `RecordModelLatency(ctx context.Context, model string, duration time.Duration)`

### `propagation/propagation.go` (package `propagation`)

- func `HTTPMiddleware(next http.Handler) http.Handler`
- func `InjectHTTP(ctx context.Context, req *http.Request)`
- func `ExtractMCP(params map[string]interface{}) context.Context`
- func `InjectMCP(ctx context.Context, params map[string]interface{}) map[string]interface{}`

### `redaction/redaction.go` (package `redaction`)

- func `Denylist() []string`
- func `ShouldRedact(key string) bool`
- func `SpanProcessor() sdktrace.SpanProcessor`
- (unexported) `redactProcessor.ShouldRedact(key string) bool`

### `internal/resource.go` (package `internal`, not part of public API)

- func `NewResource(serviceName, serviceVersion, environment string) (*resource.Resource, error)`

### `examples/hello/main.go` (package `main`)

- runnable example; uses stdout exporter, `feotel.StartSpan`, `genai.ModelCallSpan`, `genai.RecordTokenUsage`, `genai.RecordModelLatency`.

## Open Questions

1. Is the observability contract (`docs/observability-contract.md`) authoritative for the suite, or has the code intentionally diverged from it (`feotel` vs `tiamatotel`, `fragments-engine` vs `tiamat`, `FE_OTEL_*` vs `TIAMAT_OTEL_*`)? See finding 6.
2. Should `feotel.Init` also install a `MeterProvider`, given that `RegisterMetrics` exists at the same package level? See nice-to-have 3.
3. Is there a CI or release workflow that publishes this module separately from the rest of the framework? Not visible from the lib alone.
