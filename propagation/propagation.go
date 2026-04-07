// Package propagation provides W3C trace context propagation for HTTP and MCP.
package propagation

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns an http.Handler that extracts W3C traceparent
// and creates a server span for each request.
func HTTPMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer("fe.http")
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

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", sw.status))
		if sw.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
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
