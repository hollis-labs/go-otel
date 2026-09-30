package propagation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestInjectAndExtractMCP(t *testing.T) {
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{},
		otelpropagation.Baggage{},
	))

	var traceID trace.TraceID
	copy(traceID[:], []byte{
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c,
		0x0d, 0x0e, 0x0f, 0x10,
	})
	var spanID trace.SpanID
	copy(spanID[:], []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	params := InjectMCP(ctx, nil)
	if got, want := params["_traceparent"], "00-"+traceID.String()+"-"+spanID.String()+"-01"; got != want {
		t.Fatalf("_traceparent = %v, want %v", got, want)
	}

	gotCtx := ExtractMCP(context.Background(), params)
	gotSC := trace.SpanContextFromContext(gotCtx)
	if gotSC.TraceID() != traceID {
		t.Fatalf("traceID = %s, want %s", gotSC.TraceID(), traceID)
	}
	if gotSC.SpanID() != spanID {
		t.Fatalf("spanID = %s, want %s", gotSC.SpanID(), spanID)
	}
}

func TestInjectHTTPAddsTraceContext(t *testing.T) {
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{},
		otelpropagation.Baggage{},
	))

	var traceID trace.TraceID
	copy(traceID[:], []byte{
		0xaa, 0xbb, 0xcc, 0xdd,
		0xee, 0xff, 0x00, 0x11,
		0x22, 0x33, 0x44, 0x55,
		0x66, 0x77, 0x88, 0x99,
	})
	var spanID trace.SpanID
	copy(spanID[:], []byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	req := httptest.NewRequest("GET", "http://example.com", nil)
	InjectHTTP(ctx, req)

	if got, want := req.Header.Get("traceparent"), "00-"+traceID.String()+"-"+spanID.String()+"-01"; got != want {
		t.Fatalf("traceparent = %q, want %q", got, want)
	}
}

// recordedHTTPCall captures one HTTPRequest invocation for assertions.
type recordedHTTPCall struct {
	route string
	code  int
	dur   time.Duration
}

type fakeHTTPRecorder struct {
	mu    sync.Mutex
	calls []recordedHTTPCall
}

func (f *fakeHTTPRecorder) HTTPRequest(_ context.Context, route string, code int, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedHTTPCall{route: route, code: code, dur: d})
}

func TestHTTPMiddlewareEmitsMetricsWhenRecorderSet(t *testing.T) {
	rec := &fakeHTTPRecorder{}
	handler := HTTPMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		}),
		WithMetricRecorder(rec),
		WithRouteResolver(func(*http.Request) string { return "/items/:id" }),
	)

	req := httptest.NewRequest("GET", "http://example.com/items/42", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := len(rec.calls); got != 1 {
		t.Fatalf("expected 1 recorder call, got %d", got)
	}
	call := rec.calls[0]
	if call.route != "/items/:id" {
		t.Errorf("route = %q, want %q (resolver should override r.URL.Path)", call.route, "/items/:id")
	}
	if call.code != http.StatusAccepted {
		t.Errorf("status code = %d, want %d", call.code, http.StatusAccepted)
	}
	if call.dur < 0 {
		t.Errorf("duration = %v, want >= 0", call.dur)
	}
}

func TestHTTPMiddlewareSkipsMetricsWhenNoRecorder(t *testing.T) {
	handler := HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Just exercise the path; no recorder option means no metric emission.
	// The trace-only behavior is unchanged from previous releases.
	req := httptest.NewRequest("GET", "http://example.com/ok", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Result().StatusCode)
	}
}

func TestHTTPMiddlewareFallsBackToURLPathWhenNoResolver(t *testing.T) {
	rec := &fakeHTTPRecorder{}
	handler := HTTPMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		WithMetricRecorder(rec),
		// No WithRouteResolver — should fall back to r.URL.Path.
	)
	req := httptest.NewRequest("GET", "http://example.com/raw/path", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if rec.calls[0].route != "/raw/path" {
		t.Errorf("route = %q, want %q (URL.Path fallback)", rec.calls[0].route, "/raw/path")
	}
}

type ctxKey struct{}

// ExtractMCP must build on the caller's ctx. It used to start from
// context.Background(), so a handler that wrote `ctx = ExtractMCP(args)` got the
// remote SpanContext but lost cancellation, its deadline and every value the MCP
// server had put on ctx.
func TestExtractMCPPreservesCallerContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{}, otelpropagation.Baggage{},
	))

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	params := InjectMCP(trace.ContextWithSpanContext(context.Background(), sc), nil)

	deadline := time.Now().Add(time.Hour)
	parent, cancelDeadline := context.WithDeadline(context.WithValue(context.Background(), ctxKey{}, "sentinel"), deadline)
	defer cancelDeadline()
	ctx, cancel := context.WithCancel(parent)

	got := ExtractMCP(ctx, params)

	if v, _ := got.Value(ctxKey{}).(string); v != "sentinel" {
		t.Errorf("caller's context value lost: got %q", v)
	}
	if d, ok := got.Deadline(); !ok || !d.Equal(deadline) {
		t.Errorf("caller's deadline lost: got %v, %v", d, ok)
	}
	if trace.SpanContextFromContext(got).TraceID() != traceID {
		t.Error("remote trace context not extracted")
	}

	select {
	case <-got.Done():
		t.Fatal("returned ctx is already done before the caller canceled")
	default:
	}
	cancel()
	select {
	case <-got.Done():
	case <-time.After(time.Second):
		t.Fatal("canceling the caller's ctx is not observable on the ctx ExtractMCP returned")
	}
}

// With no propagator configured, extraction recovers no trace context, but it
// must still hand back the caller's own ctx.
func TestExtractMCPWithoutPropagatorKeepsCallerContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator())

	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	got := ExtractMCP(ctx, map[string]interface{}{"_traceparent": "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01"})
	if v, _ := got.Value(ctxKey{}).(string); v != "sentinel" {
		t.Errorf("caller's context value lost with no propagator: got %q", v)
	}
	if trace.SpanContextFromContext(got).IsValid() {
		t.Error("a trace context appeared with no propagator configured")
	}
}
