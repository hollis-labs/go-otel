package hotel

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hollis-labs/go-otel/internal"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Option configures Init.
type Option func(*config)

type config struct {
	serviceName          string
	serviceVersion       string
	serviceNamespace     string
	environment          string
	otlpEndpoint         string
	sampler              sdktrace.Sampler
	metricsEnabled       bool
	metricExportInterval time.Duration
	logsEnabled          bool
	runtimeMetrics       bool
	resourceOptions      []resource.Option
}

// defaultMetricExportInterval matches the SDK default; surfaced as a constant
// so the no-export smoke test and downstream operators can rely on it.
const defaultMetricExportInterval = 15 * time.Second

// Init sets up a trace provider with an OTLP HTTP exporter and, when the
// corresponding option is supplied, an OTLP HTTP metric exporter behind a
// PeriodicReader-backed MeterProvider (WithMetricsEnabled) and/or an
// OTLP HTTP log exporter behind a BatchProcessor-backed LoggerProvider
// (WithLogsEnabled).
//
// It reads from standard OTel env vars and applies any Option overrides.
// The returned shutdown function flushes and shuts down every provider
// it installed.
func Init(ctx context.Context, opts ...Option) (shutdown func(context.Context) error, err error) {
	cfg := config{
		serviceName:          envOr("OTEL_SERVICE_NAME", ""),
		serviceVersion:       envOr("OTEL_SERVICE_VERSION", "unknown"),
		serviceNamespace:     envOr("OTEL_SERVICE_NAMESPACE", ""),
		environment:          "development",
		otlpEndpoint:         envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318"),
		sampler:              nil, // means AlwaysSample
		metricExportInterval: defaultMetricExportInterval,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.sampler == nil {
		cfg.sampler = sdktrace.AlwaysSample()
	}

	res, err := internal.NewResource(ctx,
		cfg.serviceName, cfg.serviceVersion, cfg.serviceNamespace, cfg.environment,
		cfg.resourceOptions...,
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(cfg.sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdownFns := []func(context.Context) error{tp.Shutdown}

	if cfg.metricsEnabled {
		metricExporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(cfg.otlpEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			// Best-effort cleanup of the trace provider we just installed
			// so callers don't leak a live exporter on error.
			_ = tp.Shutdown(ctx)
			return nil, err
		}

		reader := sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(cfg.metricExportInterval),
		)
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		shutdownFns = append(shutdownFns, mp.Shutdown)

		if cfg.runtimeMetrics {
			// runtime.Start binds to the meter provider currently installed
			// on otel.GetMeterProvider, so it must run after the
			// SetMeterProvider call above. Errors from the runtime
			// instrumentation are non-fatal — the rest of telemetry is
			// already up and we don't want a partial init to fail Init.
			_ = runtime.Start(runtime.WithMeterProvider(mp))
		}
	}

	if cfg.logsEnabled {
		logExporter, err := otlploghttp.New(ctx,
			otlploghttp.WithEndpoint(cfg.otlpEndpoint),
			otlploghttp.WithInsecure(),
		)
		if err != nil {
			// Best-effort cleanup of providers we've already installed.
			for _, fn := range shutdownFns {
				_ = fn(ctx)
			}
			return nil, err
		}

		processor := sdklog.NewBatchProcessor(logExporter)
		lp := sdklog.NewLoggerProvider(
			sdklog.WithProcessor(processor),
			sdklog.WithResource(res),
		)
		otellog.SetLoggerProvider(lp)
		shutdownFns = append(shutdownFns, lp.Shutdown)
	}

	return func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFns {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}, nil
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

// WithServiceNamespace sets the service.namespace resource attribute. When
// empty (the default), no service.namespace attribute is emitted. May also
// be set via the OTEL_SERVICE_NAMESPACE environment variable.
func WithServiceNamespace(namespace string) Option {
	return func(c *config) { c.serviceNamespace = namespace }
}

// WithOTLPEndpoint overrides OTEL_EXPORTER_OTLP_ENDPOINT (default: "localhost:4318").
// The same endpoint serves traces on /v1/traces and, when metrics are enabled,
// metrics on /v1/metrics.
func WithOTLPEndpoint(endpoint string) Option {
	return func(c *config) { c.otlpEndpoint = endpoint }
}

// WithSampler sets the trace sampler (default: AlwaysSample).
func WithSampler(sampler sdktrace.Sampler) Option {
	return func(c *config) { c.sampler = sampler }
}

// WithMetricsEnabled installs an OTLP HTTP metric exporter and a
// PeriodicReader-backed MeterProvider on otel.SetMeterProvider, so metrics
// registered via RegisterMetrics (or any otel.Meter caller) are exported
// alongside traces.
//
// Default OFF. Endpoint resolution matches the trace exporter:
// WithOTLPEndpoint takes precedence; otherwise OTEL_EXPORTER_OTLP_ENDPOINT;
// otherwise localhost:4318. The same endpoint serves both /v1/traces and
// /v1/metrics.
//
// The PeriodicReader interval defaults to 15s; operators can override it
// with the OTEL_METRIC_EXPORT_INTERVAL environment variable (read by the
// SDK).
func WithMetricsEnabled() Option {
	return func(c *config) { c.metricsEnabled = true }
}

// WithRuntimeMetrics starts the upstream OTel Go-runtime instrumentation
// (process.runtime.go.* metrics: GC pause times, goroutine count, memory
// stats, heap allocations) against the MeterProvider installed by
// WithMetricsEnabled.
//
// Default OFF. Requires WithMetricsEnabled — if the MeterProvider is the
// no-op default, the runtime instrumentation registers against it and
// silently produces nothing. This option is a no-op when
// WithMetricsEnabled is not set; we don't return an error so callers can
// safely combine them in any order without ordering land mines.
func WithRuntimeMetrics() Option {
	return func(c *config) { c.runtimeMetrics = true }
}

// WithLogsEnabled installs an OTLP HTTP log exporter and a
// BatchProcessor-backed LoggerProvider on the global logger provider, so
// log records emitted via the OTel log API (and the slog bridge exposed
// by NewSlogHandler) are exported alongside traces and metrics.
//
// Default OFF. Endpoint resolution matches the trace/metric exporters:
// WithOTLPEndpoint takes precedence; otherwise OTEL_EXPORTER_OTLP_ENDPOINT;
// otherwise localhost:4318. The same endpoint serves /v1/traces,
// /v1/metrics, and /v1/logs.
//
// The BatchProcessor batches log records before export; operators can tune
// it via the SDK-standard OTEL_BLRP_* environment variables (read by the
// SDK).
func WithLogsEnabled() Option {
	return func(c *config) { c.logsEnabled = true }
}

// WithResourceDetectors merges attributes produced by the given
// resource.Option values into the Resource attached to every installed
// provider (traces, metrics, logs). Use this to add host, OS, process,
// container, k8s, or cloud detection.
//
// Pair with DefaultDetectors() for a sensible baseline:
//
//	hotel.Init(ctx,
//	    hotel.WithMetricsEnabled(),
//	    hotel.WithResourceDetectors(hotel.DefaultDetectors()...),
//	)
//
// Service-identity attributes (service.name, service.version,
// service.namespace, deployment.environment) always win over detector
// output so misconfigured detectors can't silently rename your service.
func WithResourceDetectors(opts ...resource.Option) Option {
	return func(c *config) {
		c.resourceOptions = append(c.resourceOptions, opts...)
	}
}

// DefaultDetectors returns a baseline set of upstream resource options
// that populate host, OS, process, and container attributes from the
// local environment without any network calls:
//
//   - resource.WithHost() — host.name, host.id, host.arch
//   - resource.WithOS() — os.type, os.description
//   - resource.WithProcess() — process.pid, process.executable.name,
//     process.command_args, process.owner, process.runtime.{name,version,
//     description}
//   - resource.WithContainer() — container.id when running inside a
//     container (reads /proc/self/cgroup; silently empty otherwise)
//
// Cloud (AWS, GCP, Azure) and k8s detectors are NOT included because they
// can issue metadata-server requests with their own timeout semantics;
// add them explicitly when running in those environments via additional
// WithResourceDetectors calls.
func DefaultDetectors() []resource.Option {
	return []resource.Option{
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithContainer(),
	}
}

// NotifyShutdown returns a context that's canceled when SIGTERM, SIGINT,
// or os.Interrupt fires. Pair with Init to wire graceful shutdown using
// idiomatic Go signal handling:
//
//	ctx, stop := hotel.NotifyShutdown()
//	defer stop()
//
//	shutdown, err := hotel.Init(ctx, ...)
//	if err != nil { log.Fatal(err) }
//	defer hotel.ShutdownWithTimeout(shutdown, 5*time.Second)
//
//	runServer(ctx) // returns when ctx is canceled
//
// The returned cancel function should be called via defer to release the
// signal handler (matches signal.NotifyContext).
func NotifyShutdown() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
}

// ShutdownWithTimeout calls shutdown with a fresh context.Background()
// bounded by the given timeout. Suitable for use inside defer:
//
//	defer hotel.ShutdownWithTimeout(shutdown, 5*time.Second)
//
// Use this when the shutdown happens during process teardown — the
// regular request/operation context is no longer suitable since it may
// already be canceled (e.g., when NotifyShutdown's context fires).
func ShutdownWithTimeout(shutdown func(context.Context) error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
