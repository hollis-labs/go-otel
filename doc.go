// Package hotel ("Hollis OTel") provides an opinionated OpenTelemetry
// bootstrap and a small set of span, metric, and log helpers built around
// the OTLP HTTP exporter family.
//
// Init wires up a batching TracerProvider with an OTLP HTTP exporter and
// installs a composite W3C TraceContext + Baggage propagator. Three opt-in
// pillars layer on top:
//
//   - WithMetricsEnabled installs an OTLP HTTP metric exporter behind a
//     PeriodicReader-backed MeterProvider so instruments registered via
//     RegisterMetrics (or any otel.Meter caller) are exported alongside
//     traces. WithRuntimeMetrics layers Go-runtime instrumentation
//     (process.runtime.go.*) on top.
//   - WithLogsEnabled installs an OTLP HTTP log exporter behind a
//     BatchProcessor-backed LoggerProvider on the global logger provider.
//     NewSlogHandler fans an slog.Logger out to both stderr and OTLP.
//   - WithResourceDetectors merges upstream resource.Option values
//     (host, OS, process, container, etc.) into the Resource attached to
//     every installed provider. DefaultDetectors returns a sensible
//     baseline that doesn't make network calls.
//
// All three pillars are off by default so existing callers do not silently
// gain new network dependencies.
//
// The package exposes helpers for the hollis.* span taxonomy this library
// promotes (hollis.agent.step, hollis.tool.call, hollis.memory.read,
// hollis.memory.write), the hollis.* metric instrument set returned by
// RegisterMetrics, and a Recorder layer (RegisterRecorder / NewRecorder)
// that binds an app label and exposes typed helpers per instrument family
// so call sites don't re-implement label discipline.
//
// NotifyShutdown wraps signal.NotifyContext bound to SIGTERM, SIGINT, and
// os.Interrupt; ShutdownWithTimeout wraps the shutdown closure returned
// by Init in a fresh deadline-bounded context for use in defer. Together
// they cover the common graceful-shutdown shape.
//
// Sub-packages cover additional surfaces:
//
//   - genai: OpenTelemetry GenAI semantic-convention helpers
//   - propagation: HTTP middleware (optionally auto-instrumented for
//     metrics via WithMetricRecorder / WithRouteResolver), HTTP injection,
//     MCP-style propagation
//   - redaction: denylist helpers for sensitive prompt/completion
//     attributes
//
// See the README and pkg.go.dev for a quickstart and the rest of the
// public API.
package hotel
