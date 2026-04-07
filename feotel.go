// Package feotel provides shared OpenTelemetry instrumentation for the
// Fragments Engine suite of applications (Engine, Conduit, Cortex, Hadron, Nanite).
package feotel

import (
	"context"
	"os"

	"github.com/hollis-labs/otel/internal"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Option configures the otel initialisation.
type Option func(*config)

type config struct {
	serviceName    string
	serviceVersion string
	environment    string
	otlpEndpoint   string
	sampler        sdktrace.Sampler
}

// Init sets up a trace provider with an OTLP HTTP exporter.
// It reads from standard OTel env vars and applies any Option overrides.
// The returned shutdown function flushes and shuts down the provider.
func Init(ctx context.Context, opts ...Option) (shutdown func(context.Context) error, err error) {
	cfg := config{
		serviceName:    envOr("OTEL_SERVICE_NAME", ""),
		serviceVersion: envOr("OTEL_SERVICE_VERSION", "unknown"),
		environment:    "development",
		otlpEndpoint:   envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
		sampler:        nil, // means AlwaysSample
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.sampler == nil {
		cfg.sampler = sdktrace.AlwaysSample()
	}

	res, err := internal.NewResource(cfg.serviceName, cfg.serviceVersion, cfg.environment)
	if err != nil {
		return nil, err
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(cfg.sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// WithServiceName overrides OTEL_SERVICE_NAME.
func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

// WithServiceVersion overrides OTEL_SERVICE_VERSION.
func WithServiceVersion(version string) Option {
	return func(c *config) { c.serviceVersion = version }
}

// WithEnvironment sets the deployment environment (default: "development").
func WithEnvironment(env string) Option {
	return func(c *config) { c.environment = env }
}

// WithOTLPEndpoint overrides OTEL_EXPORTER_OTLP_ENDPOINT (default: "localhost:4317").
func WithOTLPEndpoint(endpoint string) Option {
	return func(c *config) { c.otlpEndpoint = endpoint }
}

// WithSampler sets the trace sampler (default: AlwaysSample).
func WithSampler(sampler sdktrace.Sampler) Option {
	return func(c *config) { c.sampler = sampler }
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
