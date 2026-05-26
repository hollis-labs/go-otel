// Package hotel ("Hollis OTel") provides an opinionated OpenTelemetry
// bootstrap and a small set of span and metric helpers built around the
// OTLP HTTP exporter.
//
// Init wires up a batching TracerProvider with an OTLP HTTP exporter and
// installs a composite W3C TraceContext + Baggage propagator. When
// WithMetricsEnabled is supplied, it additionally installs an OTLP HTTP
// metric exporter behind a PeriodicReader-backed MeterProvider so that
// instruments registered via RegisterMetrics (or any otel.Meter caller)
// are exported alongside traces. Metrics are off by default so existing
// callers do not silently gain a new network dependency.
//
// The package exposes helpers for the hollis.* span taxonomy this library
// promotes (hollis.agent.step, hollis.tool.call, hollis.memory.read,
// hollis.memory.write), a slog handler that injects trace_id and span_id
// from context, and the hollis.* metric instrument set returned by
// RegisterMetrics.
//
// Sub-packages cover additional surfaces:
//
//   - genai: OpenTelemetry GenAI semantic-convention helpers
//   - propagation: HTTP middleware, HTTP injection, MCP-style propagation
//   - redaction: denylist helpers for sensitive prompt/completion attributes
//
// See the README and pkg.go.dev for a quickstart and the rest of the
// public API.
package hotel
