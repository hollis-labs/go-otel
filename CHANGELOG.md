# Changelog

All notable changes to `go-otel` are recorded in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the project is pre-1.0 (v0.x), minor releases may contain backward-incompatible
changes; breaking changes are always called out under "Changed" or "Removed".

## [v0.6.0] — 2026-05-26

Closes the inbox follow-up list with two bootstrap conveniences:
signal-aware shutdown helpers and resource-detector support. Additive
over v0.5.0; service-identity attributes still win over detector output
so misconfigured detectors can't silently rename your service.

### Added
- `WithResourceDetectors(opts ...resource.Option) Option` merges
  attributes produced by upstream `resource.Option` values into the
  Resource attached to every installed provider (traces, metrics, logs).
  Use to add host, OS, process, container, k8s, or cloud detection.
- `DefaultDetectors() []resource.Option` returns a baseline set of
  upstream resource options that populate host/OS/process/container
  attributes from the local environment without any network calls:
  - `resource.WithHost()` — `host.name`, `host.id`, `host.arch`
  - `resource.WithOS()` — `os.type`, `os.description`
  - `resource.WithProcess()` — `process.pid`,
    `process.executable.name`, `process.command_args`, `process.owner`,
    `process.runtime.{name,version,description}`
  - `resource.WithContainer()` — `container.id` when running inside a
    container (silently empty otherwise)

  Cloud (AWS/GCP/Azure) and k8s detectors are NOT included because they
  can issue metadata-server requests with their own timeout semantics;
  callers add those explicitly when running in those environments.
- `NotifyShutdown() (ctx, stop)` — convenience wrapper around
  `signal.NotifyContext` bound to SIGTERM, SIGINT, and `os.Interrupt`.
  Returns a context that's canceled on signal and a stop function to
  release the handler.
- `ShutdownWithTimeout(shutdown, timeout)` — calls shutdown with a
  fresh `context.Background()` bounded by the given timeout. Suitable
  for use inside `defer` during process teardown when the regular
  request/operation context is no longer suitable (already canceled).

### Changed
- `internal.NewResource` signature gained a leading `ctx context.Context`
  parameter and a trailing `extras ...resource.Option` variadic. The
  `internal/` package is not part of the public API; this is documented
  for completeness only. Public callers see no breakage.

## [v0.5.0] — 2026-05-26

Two app-bootstrap conveniences bundled per the follow-up plan: Go-runtime
metrics and auto-instrumented HTTP middleware. Additive over v0.4.0;
defaults preserve existing behavior.

### Added
- `WithRuntimeMetrics() Option` starts the upstream OTel Go-runtime
  instrumentation (`process.runtime.go.*` metrics: GC pause times,
  goroutine count, memory stats, heap allocations) against the
  `MeterProvider` installed by `WithMetricsEnabled`. Default OFF. No-op
  when `WithMetricsEnabled` is absent (the runtime instrumentation
  registers against the no-op global provider and silently produces
  nothing); operators can safely combine the two options in any order.
- `propagation.MiddlewareOption` configuration on `HTTPMiddleware`. The
  function signature is now variadic: existing callers
  (`HTTPMiddleware(next)`) continue to work unchanged. New options:
  - `WithMetricRecorder(r HTTPMetricRecorder)` enables per-request
    metric emission. Each request triggers
    `r.HTTPRequest(ctx, route, statusCode, duration)` after the handler
    returns. Satisfied by `*hotel.Recorder`.
  - `WithRouteResolver(f func(*http.Request) string)` plugs in your
    router's pattern accessor to produce the bounded-cardinality `route`
    label (chi.RouteContext, httprouter.Param, etc.). When omitted, the
    middleware falls back to `r.URL.Path` — cardinality-unsafe for
    production, called out in the godoc.
- `propagation.HTTPMetricRecorder` interface
  (`HTTPRequest(ctx, route, statusCode, duration)`) — minimal contract
  the middleware uses, defined in the propagation package so callers can
  implement it without importing `hotel`.

### Module hygiene
- Added `go.opentelemetry.io/contrib/instrumentation/runtime` v0.66.0,
  pinned to the v1.41.0 OTel core line.

## [v0.4.0] — 2026-05-26

Closes the three-pillar story: traces (v0.1.0) + metrics (v0.2.0) + logs
(this release). Additive over v0.3.0; default OFF.

### Added
- `WithLogsEnabled() Option` installs an OTLP HTTP log exporter behind a
  `BatchProcessor`-backed `LoggerProvider` on the global logger provider.
  Endpoint resolution matches the trace + metric exporters: explicit
  `WithOTLPEndpoint` wins, otherwise `OTEL_EXPORTER_OTLP_ENDPOINT`,
  otherwise `localhost:4318`. The same endpoint serves `/v1/traces`,
  `/v1/metrics`, and `/v1/logs`. BatchProcessor tunables (queue size,
  schedule delay, export timeout) are read from the SDK-standard
  `OTEL_BLRP_*` environment variables.
- `NewSlogHandler(scopeName string, stderrInner slog.Handler) slog.Handler`
  — fan-out `slog.Handler` that emits each record to two destinations:
  - `NewLogHandler(stderrInner)` for the existing stderr pretty-print
    path with `trace_id` / `span_id` injection.
  - `otelslog.NewHandler(scopeName)` for OTLP export through whatever
    `LoggerProvider` is currently installed. With `WithLogsEnabled`, that's
    the exporter wired up by Init; without it, the OTel no-op provider
    silently discards records.
  Use this when you want stderr logs AND OTLP log export from the same
  `slog.Logger`.

### Changed
- `Init` shutdown now also flushes the `LoggerProvider` when logs are
  enabled. Per-provider errors continue to be joined via `errors.Join`.

### Module hygiene
- Added `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp`
  v0.17.0, `go.opentelemetry.io/otel/sdk/log` v0.17.0,
  `go.opentelemetry.io/otel/log` v0.17.0, and
  `go.opentelemetry.io/contrib/bridges/otelslog` v0.16.0. All pinned to
  the v1.41.0 OTel core line for stack coherence.

## [v0.3.0] — 2026-05-26

Additive follow-up to v0.2.0. Ships the `Recorder` helper layer so callers
don't have to bake instrument-specific label discipline into every emit
site, and don't have to refactor twice (once for the v0.2.0 `Metrics`
struct rename, again for the recorder).

### Added
- `Recorder` type that wraps `*Metrics` with a bound `app` label and
  exposes typed helpers per instrument family:
  - `HTTPRequest(ctx, route, statusCode, duration)`
  - `AgentTurn(ctx, provider, runtimeKind, result, duration)`
  - `ToolCall(ctx, toolName, result, duration)` — hits both
    `hollis.tool.call.count` (with `result`) and
    `hollis.tool.call.duration` (without `result`) with the correct labels.
  - `Message(ctx, kind, result, duration)` — same asymmetric label-shape
    handling for `hollis.message.*`.
  - `ProviderTokens(ctx, provider, model, inputTokens, outputTokens)` —
    records both `hollis.provider.tokens.input` and `.output` with the
    shared `app, provider, model` label set.
  - `ContextTokenBudget(ctx, provider, model, tokensUsed)`
  - `SSEConnectionOpened(ctx, streamType)` /
    `SSEConnectionClosed(ctx, streamType)` — +1 / −1 on the
    `hollis.sse.active_connections` UpDownCounter.
  - `SSEReconnect(ctx, streamType)`
  - `QueueDepth(ctx, queueName, delta int64)` — signed delta, so callers
    can model "drained N at once" without looping.
- `NewRecorder(metrics, app) *Recorder` wraps an existing `*Metrics`.
- `RegisterRecorder(meter, app) (*Recorder, error)` is the one-step
  convenience that calls `RegisterMetrics` then wraps.
- `Recorder.Metrics() *Metrics` escape hatch returns the underlying
  instruments for callers that need a label shape the recorder doesn't
  cover. `Recorder.App() string` returns the bound app label.

## [v0.2.0] — 2026-05-26

This release adds opt-in metrics export over OTLP and renames the Go
package + on-the-wire taxonomy. Pre-1.0 minor releases may include
backward-incompatible changes; the rename pass below is breaking — see the
"Changed (rename pass)" section for migration steps.

### Added
- `WithMetricsEnabled() Option` installs an OTLP HTTP metric exporter behind
  a PeriodicReader-backed `MeterProvider` on `otel.SetMeterProvider`. Default
  OFF — opt-in app-by-app so existing apps don't silently gain a new network
  dependency. Endpoint resolution mirrors the trace exporter: explicit
  `WithOTLPEndpoint` wins, otherwise `OTEL_EXPORTER_OTLP_ENDPOINT`, otherwise
  `localhost:4318`. The same endpoint serves both `/v1/traces` and
  `/v1/metrics`. The PeriodicReader interval defaults to 15s and can be
  overridden via the SDK-standard `OTEL_METRIC_EXPORT_INTERVAL` env var.
- New `hollis.*` metric instrument set returned by `RegisterMetrics`:
  `hollis.http.request.count` / `.duration`, `hollis.agent.turn.duration`,
  `hollis.tool.call.count` / `.duration`, `hollis.message.count` /
  `.duration`, `hollis.sse.active_connections`, `hollis.sse.reconnects`,
  `hollis.queue.depth`, `hollis.provider.tokens.input` / `.output`, and
  `hollis.context.token_budget.used`. Duration histograms use exponential
  buckets `5ms…60s`; the token-budget histogram uses power-of-two buckets
  `1k…512k`. Labels are bounded; session/task/agent/message IDs are
  trace-only and must not be attached.

### Changed
- `Init` shutdown now flushes the metric provider as well when metrics are
  enabled, joining any per-provider errors with `errors.Join`.
- The `Metrics` struct returned by `RegisterMetrics` is reshaped to expose
  the new `hollis.*` instruments. Field names change accordingly
  (`HTTPRequestCount`, `HTTPRequestDuration`, `AgentTurnDuration`,
  `ToolCallCount`, `ToolCallDuration`, `MessageCount`, `MessageDuration`,
  `SSEActiveConnections`, `SSEReconnects`, `QueueDepth`,
  `ProviderTokensInput`, `ProviderTokensOutput`, `ContextTokenBudgetUsed`).
  This is a breaking change for callers that referenced
  `.RequestCount` / `.RequestLatency` / `.ErrorCount`.

### Removed
- The `fe.request.count`, `fe.request.latency`, and `fe.error.count`
  instruments. They are subsumed by `hollis.http.request.count` /
  `.duration` (errors are recorded via the `status_code` / `result` labels
  on the new counters).

### Changed (rename pass)
- Go package renamed `feotel` → `hotel` ("Hollis OTel"). The Go-side
  identifier is short and provider-neutral; the wire-format taxonomy
  (`hollis.*`) carries the branding. Migration: replace
  `feotel.X` call sites with `hotel.X` and drop any `feotel` import
  alias — the unaliased import path `"github.com/hollis-labs/go-otel"`
  now resolves to package `hotel`. File `feotel.go` renamed to `hotel.go`.
- Span names renamed: `fe.agent.step` → `hollis.agent.step`,
  `fe.tool.call` → `hollis.tool.call`, `fe.memory.read` →
  `hollis.memory.read`, `fe.memory.write` → `hollis.memory.write`.
- Span attribute keys renamed: `fe.agent.step.name` →
  `hollis.agent.step.name`, `fe.tool.name` → `hollis.tool.name`,
  `fe.memory.namespace` → `hollis.memory.namespace`, `fe.memory.key` →
  `hollis.memory.key`.
- HTTP middleware tracer name renamed: `fe.http` → `hollis.http`.
- Environment variable renamed: `FE_OTEL_REDACT_PROMPTS` →
  `HOLLIS_OTEL_REDACT_PROMPTS`. Operators that set the old variable must
  rename it; the library does not read both.

## [v0.1.0] — 2026-05-10

First public release.

### Added
- `WithServiceNamespace(namespace string) Option` for setting the
  `service.namespace` resource attribute. Also reads from the
  `OTEL_SERVICE_NAMESPACE` environment variable. When empty (the default),
  no `service.namespace` attribute is emitted.
- Public `README.md`, `CHANGELOG.md`, `doc.go`, and `.gitignore`.

### Changed
- `service.namespace` is no longer hardcoded. Previous releases set it to
  `"fragments-engine"` unconditionally; this is now configurable via
  `WithServiceNamespace` or `OTEL_SERVICE_NAMESPACE` and defaults to unset.
  Migration: callers that relied on the previous default should add
  `feotel.WithServiceNamespace("fragments-engine")` (or the appropriate
  namespace for their deployment) to their `Init` call.
- Package-level godoc on `feotel` rewritten to describe the public API.
- `internal.NewResource` now takes a `serviceNamespace` parameter and
  emits the attribute only when non-empty. The `internal/` package is
  not part of the public API.

### Removed
- Internal repository configuration files that should not ship in a
  public release: `.agentrc/`, `.claude/`, `CLAUDE.md`, `agentrc.yaml`,
  `AUDIT_RESULTS.md`, `lefthook.yml`, and the internal-only
  `docs/observability-contract.md`.

### Security
- Bumped `golang.org/x/net` v0.50.0 → v0.54.0 to pick up the fixes for
  GO-2026-4918 (HTTP/2 transport infinite loop on bad
  `SETTINGS_MAX_FRAME_SIZE`) and GO-2026-4559 (HTTP/2 server panic on
  certain frames). Indirect bumps of `golang.org/x/sys` and
  `golang.org/x/text` followed via `go mod tidy`.

### Fixed
- `go.mod` `go` directive bumped from 1.25.0 to 1.26.1 to match the
  current toolchain. Build / vet / `go test -race` / `govulncheck` of
  in-lib symbols are clean on Go 1.26.1; remaining govulncheck findings
  are all standard-library issues fixed in Go 1.26.2 / 1.26.3
  (`crypto/tls`, `crypto/x509`, `net`, `net/http`, `html/template`) and
  are not actionable at the library level — consumers should compile
  against Go 1.26.3 or later.
