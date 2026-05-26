// Package propagation provides W3C trace context propagation for HTTP and MCP.
package propagation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMetricRecorder is the minimal interface the HTTP middleware uses to
// emit per-request metrics. It is satisfied by *hotel.Recorder; the
// interface lives here so the propagation package does not have to import
// the hotel package (avoiding a circular-dep risk and keeping propagation
// usable as a standalone surface).
type HTTPMetricRecorder interface {
	HTTPRequest(ctx context.Context, route string, statusCode int, d time.Duration)
}

// MiddlewareOption configures HTTPMiddleware.
type MiddlewareOption func(*middlewareConfig)

type middlewareConfig struct {
	recorder      HTTPMetricRecorder
	routeResolver func(*http.Request) string
}

// WithMetricRecorder enables per-request metric emission through the given
// recorder. Each request triggers a call to
// HTTPRequest(ctx, route, statusCode, duration) after the handler returns.
// When omitted, the middleware records spans only (legacy behavior).
func WithMetricRecorder(r HTTPMetricRecorder) MiddlewareOption {
	return func(c *middlewareConfig) { c.recorder = r }
}

// WithRouteResolver supplies a function that turns *http.Request into the
// bounded-cardinality route pattern used as the route label on emitted
// metrics. Plug in your router's pattern accessor (chi.RouteContext,
// httprouter.Param, etc.) — using the raw r.URL.Path (the default
// fallback) is cardinality-unsafe in production and only acceptable for
// fixed-path APIs.
func WithRouteResolver(f func(*http.Request) string) MiddlewareOption {
	return func(c *middlewareConfig) { c.routeResolver = f }
}

// HTTPMiddleware returns an http.Handler that extracts W3C traceparent and
// creates a server span for each request. When called with a
// WithMetricRecorder option, it also emits hollis.http.request.count and
// hollis.http.request.duration through the recorder.
func HTTPMiddleware(next http.Handler, opts ...MiddlewareOption) http.Handler {
	cfg := middlewareConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	tracer := otel.Tracer("hollis.http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prop := otel.GetTextMapPropagator()
		ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		spanName := r.Method + " " + r.URL.Path
		ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.target", r.URL.Path),
		)

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))
		elapsed := time.Since(start)

		span.SetAttributes(attribute.Int("http.status_code", sw.status))
		if sw.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
		}

		if cfg.recorder != nil {
			route := r.URL.Path
			if cfg.routeResolver != nil {
				route = cfg.routeResolver(r)
			}
			cfg.recorder.HTTPRequest(ctx, route, sw.status, elapsed)
		}
	})
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the underlying ResponseWriter if it supports http.Flusher.
// Required for SSE endpoints to work through this middleware.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// InjectHTTP injects trace context into outgoing HTTP request headers.
func InjectHTTP(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// ExtractMCP extracts trace context from MCP tool call parameters.
// It looks for a "_traceparent" key in the params map.
func ExtractMCP(params map[string]interface{}) context.Context {
	carrier := propagation.MapCarrier{}
	if tp, ok := params["_traceparent"].(string); ok {
		carrier.Set("traceparent", tp)
	}
	if ts, ok := params["_tracestate"].(string); ok {
		carrier.Set("tracestate", ts)
	}
	return otel.GetTextMapPropagator().Extract(context.Background(), carrier)
}

// InjectMCP injects trace context into MCP tool call parameters.
func InjectMCP(ctx context.Context, params map[string]interface{}) map[string]interface{} {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return params
	}
	if params == nil {
		params = make(map[string]interface{})
	}
	// Build W3C traceparent: version-traceid-spanid-flags
	flags := "00"
	if sc.IsSampled() {
		flags = "01"
	}
	params["_traceparent"] = fmt.Sprintf("00-%s-%s-%s",
		sc.TraceID().String(), sc.SpanID().String(), flags)
	if sc.TraceState().Len() > 0 {
		params["_tracestate"] = sc.TraceState().String()
	}
	return params
}
