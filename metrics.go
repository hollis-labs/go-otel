package hotel

import (
	"go.opentelemetry.io/otel/metric"
)

// durationBucketsMs is the recommended exponential bucket set for latency
// histograms (milliseconds), matching the shapes seen for HTTP requests and
// agent turns in practice.
var durationBucketsMs = []float64{
	5, 10, 25, 50, 100, 250, 500,
	1000, 2500, 5000, 10000, 30000, 60000,
}

// tokenBudgetBuckets is the recommended power-of-two bucket set for
// context-window token-budget histograms (tokens), ranging from ~1k up to
// 512k to cover frontier model context sizes.
var tokenBudgetBuckets = []float64{
	1024, 2048, 4096, 8192, 16384, 32768,
	65536, 131072, 262144, 524288,
}

// Metrics holds the standard hollis.* metric instruments.
//
// Cardinality discipline: instruments here intentionally accept only
// bounded-cardinality labels (app, route, status_code, provider, model,
// kind, result, tool_name, stream_type, queue_name, runtime_kind). Session
// IDs, task IDs, agent IDs, message IDs are trace-only and MUST NOT be
// attached to these instruments.
type Metrics struct {
	// HTTP
	HTTPRequestCount    metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram

	// Agent turns
	AgentTurnDuration metric.Float64Histogram

	// Tool calls
	ToolCallCount    metric.Int64Counter
	ToolCallDuration metric.Float64Histogram

	// Message broker (send + consume share a counter via the kind label)
	MessageCount    metric.Int64Counter
	MessageDuration metric.Float64Histogram

	// SSE
	SSEActiveConnections metric.Int64UpDownCounter
	SSEReconnects        metric.Int64Counter

	// Queue depth
	QueueDepth metric.Int64UpDownCounter

	// Provider token usage
	ProviderTokensInput  metric.Int64Counter
	ProviderTokensOutput metric.Int64Counter

	// Context window budget usage
	ContextTokenBudgetUsed metric.Float64Histogram
}

// RegisterMetrics creates and returns the standard hollis.* metric
// instruments. The supplied meter is typically obtained via
// otel.Meter("<service-name>") so the instruments are associated with the
// caller's instrumentation scope; the underlying MeterProvider is the one
// installed by Init (which is a no-op unless WithMetricsEnabled was passed).
func RegisterMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{}
	var err error

	if m.HTTPRequestCount, err = meter.Int64Counter(
		"hollis.http.request.count",
		metric.WithDescription("Total HTTP requests handled. Labels: app, route, status_code."),
	); err != nil {
		return nil, err
	}

	if m.HTTPRequestDuration, err = meter.Float64Histogram(
		"hollis.http.request.duration",
		metric.WithDescription("HTTP request duration. Labels: app, route, status_code."),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(durationBucketsMs...),
	); err != nil {
		return nil, err
	}

	if m.AgentTurnDuration, err = meter.Float64Histogram(
		"hollis.agent.turn.duration",
		metric.WithDescription("Agent turn duration. Labels: app, provider, runtime_kind, result."),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(durationBucketsMs...),
	); err != nil {
		return nil, err
	}

	if m.ToolCallCount, err = meter.Int64Counter(
		"hollis.tool.call.count",
		metric.WithDescription("Total tool invocations. Labels: app, tool_name, result."),
	); err != nil {
		return nil, err
	}

	if m.ToolCallDuration, err = meter.Float64Histogram(
		"hollis.tool.call.duration",
		metric.WithDescription("Tool call duration. Labels: app, tool_name."),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(durationBucketsMs...),
	); err != nil {
		return nil, err
	}

	if m.MessageCount, err = meter.Int64Counter(
		"hollis.message.count",
		metric.WithDescription("Total broker messages (send + consume share via kind). Labels: app, kind, result."),
	); err != nil {
		return nil, err
	}

	if m.MessageDuration, err = meter.Float64Histogram(
		"hollis.message.duration",
		metric.WithDescription("Broker message handling duration. Labels: app, kind."),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(durationBucketsMs...),
	); err != nil {
		return nil, err
	}

	if m.SSEActiveConnections, err = meter.Int64UpDownCounter(
		"hollis.sse.active_connections",
		metric.WithDescription("Currently active SSE connections. Labels: app, stream_type."),
	); err != nil {
		return nil, err
	}

	if m.SSEReconnects, err = meter.Int64Counter(
		"hollis.sse.reconnects",
		metric.WithDescription("Total SSE reconnects. Labels: app, stream_type."),
	); err != nil {
		return nil, err
	}

	if m.QueueDepth, err = meter.Int64UpDownCounter(
		"hollis.queue.depth",
		metric.WithDescription("Current queue depth. Labels: app, queue_name."),
	); err != nil {
		return nil, err
	}

	if m.ProviderTokensInput, err = meter.Int64Counter(
		"hollis.provider.tokens.input",
		metric.WithDescription("Total input tokens consumed. Labels: app, provider, model."),
	); err != nil {
		return nil, err
	}

	if m.ProviderTokensOutput, err = meter.Int64Counter(
		"hollis.provider.tokens.output",
		metric.WithDescription("Total output tokens generated. Labels: app, provider, model."),
	); err != nil {
		return nil, err
	}

	if m.ContextTokenBudgetUsed, err = meter.Float64Histogram(
		"hollis.context.token_budget.used",
		metric.WithDescription("Context-window token-budget usage per turn. Labels: app, provider, model."),
		metric.WithUnit("token"),
		metric.WithExplicitBucketBoundaries(tokenBudgetBuckets...),
	); err != nil {
		return nil, err
	}

	return m, nil
}
