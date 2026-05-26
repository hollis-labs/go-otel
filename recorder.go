package hotel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Recorder is a thin wrapper around *Metrics that binds an app label and
// exposes typed helpers per instrument family. It exists so call sites don't
// re-implement label discipline: which labels belong on count vs duration,
// the +1/-1 SSE connection-gauge dance, the signed QueueDepth delta, the
// shared input/output token-counter labels, etc.
//
// Use the underlying *Metrics directly via Metrics() only when you need a
// label shape the recorder doesn't cover.
type Recorder struct {
	m   *Metrics
	app string
}

// NewRecorder wraps an existing *Metrics with the given app label. Pair
// with RegisterMetrics when you want to share one Metrics across multiple
// recorders (rare), or use RegisterRecorder for the common one-step path.
func NewRecorder(metrics *Metrics, app string) *Recorder {
	return &Recorder{m: metrics, app: app}
}

// RegisterRecorder registers the hollis.* instrument set against meter and
// returns a Recorder bound to app. Equivalent to:
//
//	metrics, err := hotel.RegisterMetrics(meter)
//	if err != nil {
//	    return nil, err
//	}
//	return hotel.NewRecorder(metrics, app), nil
func RegisterRecorder(meter metric.Meter, app string) (*Recorder, error) {
	metrics, err := RegisterMetrics(meter)
	if err != nil {
		return nil, err
	}
	return NewRecorder(metrics, app), nil
}

// Metrics returns the underlying *Metrics handle for callers that need
// direct instrument access (custom label sets, attribute reuse for hot
// paths, etc.).
func (r *Recorder) Metrics() *Metrics { return r.m }

// App returns the app label this recorder was bound to.
func (r *Recorder) App() string { return r.app }

// HTTPRequest records one HTTP request: increments hollis.http.request.count
// by 1 and records its duration on hollis.http.request.duration. Labels:
// app, route, status_code. The route argument should be the matched route
// pattern (not the raw URL path) to keep cardinality bounded.
func (r *Recorder) HTTPRequest(ctx context.Context, route string, statusCode int, d time.Duration) {
	attrs := metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("route", route),
		attribute.Int("status_code", statusCode),
	)
	r.m.HTTPRequestCount.Add(ctx, 1, attrs)
	r.m.HTTPRequestDuration.Record(ctx, float64(d.Milliseconds()), attrs)
}

// AgentTurn records one agent-turn duration on hollis.agent.turn.duration.
// Labels: app, provider, runtime_kind, result. The histogram's count
// component captures the turn count automatically.
func (r *Recorder) AgentTurn(ctx context.Context, provider, runtimeKind, result string, d time.Duration) {
	r.m.AgentTurnDuration.Record(ctx, float64(d.Milliseconds()),
		metric.WithAttributes(
			attribute.String("app", r.app),
			attribute.String("provider", provider),
			attribute.String("runtime_kind", runtimeKind),
			attribute.String("result", result),
		))
}

// ToolCall records one tool invocation: increments hollis.tool.call.count
// by 1 (with result label) and records its duration on
// hollis.tool.call.duration (without result label, per the documented
// instrument shape).
func (r *Recorder) ToolCall(ctx context.Context, toolName, result string, d time.Duration) {
	r.m.ToolCallCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("tool_name", toolName),
		attribute.String("result", result),
	))
	r.m.ToolCallDuration.Record(ctx, float64(d.Milliseconds()),
		metric.WithAttributes(
			attribute.String("app", r.app),
			attribute.String("tool_name", toolName),
		))
}

// Message records one broker message: increments hollis.message.count by 1
// (with result label) and records its duration on hollis.message.duration
// (without result label). Set kind to differentiate send vs consume.
func (r *Recorder) Message(ctx context.Context, kind, result string, d time.Duration) {
	r.m.MessageCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("kind", kind),
		attribute.String("result", result),
	))
	r.m.MessageDuration.Record(ctx, float64(d.Milliseconds()),
		metric.WithAttributes(
			attribute.String("app", r.app),
			attribute.String("kind", kind),
		))
}

// ProviderTokens records input and output token counts on
// hollis.provider.tokens.input and hollis.provider.tokens.output. Labels:
// app, provider, model. Both counters share the same label set.
//
// Zero values are still recorded so dashboards can distinguish absent from
// zero — skip the call entirely if you have neither.
func (r *Recorder) ProviderTokens(ctx context.Context, provider, model string, inputTokens, outputTokens int64) {
	attrs := metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("provider", provider),
		attribute.String("model", model),
	)
	r.m.ProviderTokensInput.Add(ctx, inputTokens, attrs)
	r.m.ProviderTokensOutput.Add(ctx, outputTokens, attrs)
}

// ContextTokenBudget records a turn's context-window token-budget usage on
// hollis.context.token_budget.used. Labels: app, provider, model.
func (r *Recorder) ContextTokenBudget(ctx context.Context, provider, model string, tokensUsed int64) {
	r.m.ContextTokenBudgetUsed.Record(ctx, float64(tokensUsed),
		metric.WithAttributes(
			attribute.String("app", r.app),
			attribute.String("provider", provider),
			attribute.String("model", model),
		))
}

// SSEConnectionOpened adds 1 to hollis.sse.active_connections for the given
// stream type. Pair with SSEConnectionClosed when the connection terminates
// so the gauge converges.
func (r *Recorder) SSEConnectionOpened(ctx context.Context, streamType string) {
	r.m.SSEActiveConnections.Add(ctx, 1, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("stream_type", streamType),
	))
}

// SSEConnectionClosed subtracts 1 from hollis.sse.active_connections for
// the given stream type.
func (r *Recorder) SSEConnectionClosed(ctx context.Context, streamType string) {
	r.m.SSEActiveConnections.Add(ctx, -1, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("stream_type", streamType),
	))
}

// SSEReconnect increments hollis.sse.reconnects by 1 for the given stream
// type.
func (r *Recorder) SSEReconnect(ctx context.Context, streamType string) {
	r.m.SSEReconnects.Add(ctx, 1, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("stream_type", streamType),
	))
}

// QueueDepth adjusts hollis.queue.depth by delta for the named queue.
// Pass a positive delta on enqueue and a negative delta on dequeue; the
// signed argument lets callers model "drained N at once" without looping.
func (r *Recorder) QueueDepth(ctx context.Context, queueName string, delta int64) {
	r.m.QueueDepth.Add(ctx, delta, metric.WithAttributes(
		attribute.String("app", r.app),
		attribute.String("queue_name", queueName),
	))
}
