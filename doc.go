// Package feotel provides an opinionated OpenTelemetry bootstrap and a
// small set of span helpers built around the OTLP HTTP exporter.
//
// Init wires up a batching TracerProvider with an OTLP HTTP exporter and
// installs a composite W3C TraceContext + Baggage propagator. The package
// also exposes helpers for the conventional fe.* span taxonomy this
// library promotes (fe.agent.step, fe.tool.call, fe.memory.read,
// fe.memory.write) and a slog handler that injects trace_id and span_id
// from context.
//
// Sub-packages cover additional surfaces:
//
//   - genai: OpenTelemetry GenAI semantic-convention helpers
//   - propagation: HTTP middleware, HTTP injection, MCP-style propagation
//   - redaction: denylist helpers for sensitive prompt/completion attributes
//
// See the README and pkg.go.dev for a quickstart and the rest of the
// public API.
package feotel
