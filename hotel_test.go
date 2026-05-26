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

	"github.com/hollis-labs/go-otel/internal"

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

func TestInitWithRuntimeMetricsCoexists(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Pair WithRuntimeMetrics with WithMetricsEnabled — the runtime
	// instrumentation needs a real MeterProvider to register against.
	shutdown, err := Init(ctx,
		WithServiceName("runtime-test-service"),
		WithOTLPEndpoint(endpoint),
		WithMetricsEnabled(),
		WithRuntimeMetrics(),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.metricPosts.Load(); got == 0 {
		t.Fatalf("expected at least one /v1/metrics POST (runtime instruments register synchronously), got 0")
	}
}

func TestInitWithRuntimeMetricsIsNoOpWithoutMetricsEnabled(t *testing.T) {
	counter := &pathCountingServer{}
	server := httptest.NewServer(counter.handler())
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Documented contract: WithRuntimeMetrics is a no-op when
	// WithMetricsEnabled is absent. Must not error, must not panic, must
	// not emit anything.
	shutdown, err := Init(ctx,
		WithServiceName("runtime-noop-test"),
		WithOTLPEndpoint(endpoint),
		WithRuntimeMetrics(),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}

	if got := counter.metricPosts.Load(); got != 0 {
		t.Fatalf("expected 0 /v1/metrics POSTs without WithMetricsEnabled, got %d", got)
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

func TestNewResourceWithDetectorsPopulatesHostAttributes(t *testing.T) {
	// Build a resource directly via the internal helper using
	// DefaultDetectors and assert at least one host/process attribute
	// landed in the resulting resource. The set of attributes detectors
	// produce is platform-dependent, so we check for the *category*
	// rather than a specific key.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := internal.NewResource(ctx, "detector-test", "0.0.1", "", "test", DefaultDetectors()...)
	if err != nil {
		t.Fatalf("build resource error = %v", err)
	}

	keys := make(map[string]string, res.Len())
	for _, kv := range res.Attributes() {
		keys[string(kv.Key)] = kv.Value.Emit()
	}

	// Service identity must still win — that's the precedence contract.
	if got := keys["service.name"]; got != "detector-test" {
		t.Errorf("service.name = %q, want %q (service identity should outrank detectors)", got, "detector-test")
	}

	// At least one detected category should appear. host.name or
	// process.pid are both very reliable across platforms; we check both
	// and require one.
	if _, hasHost := keys["host.name"]; !hasHost {
		if _, hasProc := keys["process.pid"]; !hasProc {
			t.Fatalf("expected host.name or process.pid in resource attrs, got keys: %v", keys)
		}
	}
}

func TestShutdownWithTimeoutInvokesShutdown(t *testing.T) {
	called := 0
	var seenDeadline bool
	fake := func(ctx context.Context) error {
		called++
		_, seenDeadline = ctx.Deadline()
		return nil
	}
	if err := ShutdownWithTimeout(fake, 100*time.Millisecond); err != nil {
		t.Fatalf("ShutdownWithTimeout() error = %v", err)
	}
	if called != 1 {
		t.Errorf("shutdown called %d times, want 1", called)
	}
	if !seenDeadline {
		t.Error("shutdown was called without a deadline; expected the helper to bound it")
	}
}

func TestNotifyShutdownReturnsCancellableContext(t *testing.T) {
	ctx, stop := NotifyShutdown()
	defer stop()

	if ctx == nil {
		t.Fatal("NotifyShutdown returned nil context")
	}
	// Sanity: stop() should propagate cancellation to ctx.
	stop()
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(1 * time.Second):
		t.Fatal("ctx.Done() not signaled after stop()")
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
