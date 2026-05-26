# Changelog

All notable changes to `go-otel` are recorded in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the project is pre-1.0 (v0.x), minor releases may contain backward-incompatible
changes; breaking changes are always called out under "Changed" or "Removed".

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
