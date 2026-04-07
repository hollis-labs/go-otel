---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Design Decisions

## 1. Pure Go library, no runtime dependencies

**Decision**: otel is a Go module with no binary, no config files, no database. It is imported and called by consumer applications.

**Rationale**: Keeps the instrumentation layer zero-cost to adopt. Consumers add one import and one `Init()` call. No sidecar, no agent process, no deployment artifact.

## 2. Contract-first design

**Decision**: The observability contract (`docs/observability-contract.md`) was written before the code. All span names, attribute keys, and env vars are specified in the contract, and the library implements them.

**Rationale**: Ensures consistency across 6 projects. The contract serves as the single source of truth for what telemetry looks like, independent of which project emits it.

## 3. Shared module via replace directive (not registry)

**Decision**: All consumers reference otel via `replace github.com/hollis-labs/otel => ../tiamat-otel` in `go.mod` rather than publishing to a Go module proxy.

**Rationale**: The Fragments Engine suite is developed locally in a monorepo-adjacent layout (`~/Projects-apps/`). Publishing adds friction during rapid iteration. The replace directive gives instant feedback when the library changes. Registry publishing is deferred until the API stabilizes.

## 4. OTel SDK version pinning at v1.41.0

**Decision**: All OTel dependencies are pinned to v1.41.0.

**Rationale**: Ensures all consumers use the same OTel version, avoiding diamond dependency conflicts. Upgrades are done in otel first, then propagated to consumers via `go mod tidy`.

## 5. OTLP HTTP exporter (not gRPC)

**Decision**: `Init()` uses `otlptracehttp` (HTTP/protobuf) as the default exporter, with endpoint defaulting to `localhost:4318`.

**Rationale**: HTTP is simpler to set up for local development (no gRPC port conflicts, works with more collectors out of the box). The contract mentions gRPC as an option but the implementation defaults to HTTP.

## 6. Redaction on by default

**Decision**: GenAI prompt and completion content is redacted by default. Opt out via `TIAMAT_OTEL_REDACT_PROMPTS=false`.

**Rationale**: Safety-first. Prompts may contain user data, API keys, or sensitive instructions. Developers must explicitly opt in to see content in traces.

## 7. MCP propagation via _traceparent key

**Decision**: Trace context is propagated through MCP tool call parameters using a `_traceparent` key (underscore prefix), following W3C traceparent format.

**Rationale**: MCP tool calls pass JSON parameters, not HTTP headers. The underscore prefix signals the key is infrastructure metadata, not a tool parameter. The W3C format ensures interoperability with standard OTel propagators.

## 8. Functional options pattern for Init()

**Decision**: `Init()` accepts variadic `Option` functions (`WithServiceName`, `WithEnvironment`, etc.) that override env var defaults.

**Rationale**: Standard Go API design. Allows zero-config usage (env vars only) while supporting programmatic overrides for testing and embedding.
