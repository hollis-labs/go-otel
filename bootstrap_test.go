package hotel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
)

type logRecorder struct {
	mu    sync.Mutex
	lines []string
}

func (r *logRecorder) logf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func (r *logRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

// failingDetector is a resource detector whose detection always fails, which
// makes Init return an error without touching the network.
type failingDetector struct{}

func (failingDetector) Detect(context.Context) (*resource.Resource, error) {
	return nil, fmt.Errorf("detector boom")
}

func TestInitOrWarn_FailureLogsAndReturnsNoop(t *testing.T) {
	rec := &logRecorder{}
	shutdown := InitOrWarn(context.Background(), rec.logf, time.Second,
		WithResourceDetectors(resource.WithDetectors(failingDetector{})),
	)
	if shutdown == nil {
		t.Fatal("InitOrWarn returned a nil shutdown on failure; callers defer it unconditionally")
	}
	lines := rec.snapshot()
	if len(lines) != 1 || !strings.Contains(lines[0], "OTel init failed") || !strings.Contains(lines[0], "detector boom") {
		t.Fatalf("log lines = %q, want one 'OTel init failed' line carrying the cause", lines)
	}
	shutdown() // must be safe to call, and side-effect-free
	shutdown()
	if got := rec.snapshot(); len(got) != 1 {
		t.Errorf("the no-op shutdown logged: %q", got)
	}
}

func TestInitOrWarn_SuccessShutdownFlushesQuietly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	rec := &logRecorder{}
	shutdown := InitOrWarn(context.Background(), rec.logf, 5*time.Second,
		WithServiceName("bootstrap-test"),
		WithOTLPEndpoint(strings.TrimPrefix(server.URL, "http://")),
	)
	_, span := StartSpan(context.Background(), "bootstrap.test")
	span.End()
	shutdown()
	if lines := rec.snapshot(); len(lines) != 0 {
		t.Errorf("a clean init and shutdown logged: %q", lines)
	}
}

// The reason InitOrWarn always binds a timeout: an exporter that hangs on flush
// must not hang process exit. The endpoint below never answers; shutdown has to
// return once the timeout passes, and report it.
func TestInitOrWarn_ShutdownIsBoundedWhenTheExporterHangs(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })

	rec := &logRecorder{}
	const timeout = 300 * time.Millisecond
	shutdown := InitOrWarn(context.Background(), rec.logf, timeout,
		WithServiceName("bootstrap-hang-test"),
		WithOTLPEndpoint(strings.TrimPrefix(server.URL, "http://")),
	)
	_, span := StartSpan(context.Background(), "bootstrap.hang")
	span.End()

	done := make(chan struct{})
	start := time.Now()
	go func() { shutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown did not return: the exporter's hang is not bounded by the timeout")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("shutdown took %v with a %v timeout", elapsed, timeout)
	}
	lines := rec.snapshot()
	if len(lines) != 1 || !strings.Contains(lines[0], "OTel shutdown failed") {
		t.Errorf("log lines = %q, want one 'OTel shutdown failed' line", lines)
	}
}

func TestEnvironmentFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		hollis, app   string
		appVar        string
		fallback, out string
	}{
		{"HOLLIS_ENV wins over the app var", "staging", "prod", "TESTAPP_ENV", "development", "staging"},
		{"app var wins when HOLLIS_ENV unset", "", "prod", "TESTAPP_ENV", "development", "prod"},
		{"fallback when neither is set", "", "", "TESTAPP_ENV", "development", "development"},
		{"whitespace-only HOLLIS_ENV is unset", "   ", "prod", "TESTAPP_ENV", "development", "prod"},
		{"whitespace-only app var is unset", "", " \t ", "TESTAPP_ENV", "development", "development"},
		{"values are trimmed", "  uat \n", "", "TESTAPP_ENV", "development", "uat"},
		{"empty appEnvVar skips that tier", "", "prod", "", "dev", "dev"},
		{"HOLLIS_ENV still wins with no app var name", "staging", "prod", "", "dev", "staging"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOLLIS_ENV", tc.hollis)
			t.Setenv("TESTAPP_ENV", tc.app)
			if got := EnvironmentFromEnv(tc.appVar, tc.fallback); got != tc.out {
				t.Errorf("EnvironmentFromEnv(%q, %q) = %q, want %q", tc.appVar, tc.fallback, got, tc.out)
			}
		})
	}
}
