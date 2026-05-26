package hotel

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

func TestInitAndShutdownWithLocalOTLPHTTPServer(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("test-service"),
		WithServiceVersion("1.2.3"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	spanCtx, span := StartSpan(ctx, "test.span")
	span.End()
	_ = spanCtx

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

// pathCountingServer counts POST requests to specific OTLP paths so tests
// can assert which exporters fired.
type pathCountingServer struct {
	mu          sync.Mutex
	tracePosts  atomic.Int32
	metricPosts atomic.Int32
	logPosts    atomic.Int32
}

func (p *pathCountingServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		if r.Method == http.MethodPost {
			switch r.URL.Path {
			case "/v1/traces":
				p.tracePosts.Add(1)
			case "/v1/metrics":
				p.metricPosts.Add(1)
			case "/v1/logs":
				p.logPosts.Add(1)
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestInitWithMetricsEnabledExportsToOTLPMetrics(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("metrics-test-service"),
		WithServiceVersion("0.0.1"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
		WithMetricsEnabled(),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Register a hollis.* instrument set against the now-real MeterProvider
	// and emit a data point, mirroring real app usage.
	m, err := RegisterMetrics(otel.Meter("metrics-test"))
	if err != nil {
		t.Fatalf("RegisterMetrics() error = %v", err)
	}
	m.HTTPRequestCount.Add(ctx, 1)

	// Shutdown forces a flush of the PeriodicReader; we don't need to wait
	// for the interval to elapse.
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.metricPosts.Load(); got == 0 {
		t.Fatalf("expected at least one POST to /v1/metrics, got 0 (trace POSTs = %d)", counter.tracePosts.Load())
	}
}

func TestInitWithoutMetricsDoesNotExportMetrics(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("no-metrics-test-service"),
		WithServiceVersion("0.0.1"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
		// Note: WithMetricsEnabled deliberately omitted.
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Recording against the global meter must be a no-op when metrics are
	// disabled. We do this through the public API to confirm there's no
	// hidden MeterProvider that would emit.
	histogram, err := otel.Meter("no-metrics-test").Float64Histogram("smoke.histogram",
		metric.WithDescription("should not export"),
	)
	if err != nil {
		t.Fatalf("Float64Histogram() error = %v", err)
	}
	histogram.Record(ctx, 1)

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.metricPosts.Load(); got != 0 {
		t.Fatalf("expected 0 POSTs to /v1/metrics with metrics disabled, got %d", got)
	}
}

func TestInitWithLogsEnabledExportsToOTLPLogs(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("logs-test-service"),
		WithServiceVersion("0.0.1"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
		WithLogsEnabled(),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Emit a log record through the fan-out slog handler. The OTLP side
	// runs through the global LoggerProvider that Init just installed.
	logger := slog.New(NewSlogHandler("logs-test", slog.NewTextHandler(os.Stderr, nil)))
	logger.InfoContext(ctx, "smoke log record", slog.String("test", "logs-export"))

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.logPosts.Load(); got == 0 {
		t.Fatalf("expected at least one POST to /v1/logs, got 0 (trace POSTs = %d)", counter.tracePosts.Load())
	}
}

func TestInitWithoutLogsDoesNotExportLogs(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("no-logs-test-service"),
		WithServiceVersion("0.0.1"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
		// Note: WithLogsEnabled deliberately omitted.
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Fan-out handler is safe to use without WithLogsEnabled — the OTLP
	// side falls through to the no-op global LoggerProvider.
	logger := slog.New(NewSlogHandler("no-logs-test", slog.NewTextHandler(os.Stderr, nil)))
	logger.InfoContext(ctx, "should not export over OTLP")

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.logPosts.Load(); got != 0 {
		t.Fatalf("expected 0 POSTs to /v1/logs with logs disabled, got %d", got)
	}
}
