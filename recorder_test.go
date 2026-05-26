package hotel

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// recorderTestRig builds an isolated MeterProvider + ManualReader and
// returns a Recorder bound to "test-app" against it. Each test gets its
// own rig so collected data doesn't leak between cases.
type recorderTestRig struct {
	reader *sdkmetric.ManualReader
	rec    *Recorder
}

func newRecorderTestRig(t *testing.T) *recorderTestRig {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	rec, err := RegisterRecorder(provider.Meter("recorder-test"), "test-app")
	if err != nil {
		t.Fatalf("RegisterRecorder() error = %v", err)
	}
	return &recorderTestRig{reader: reader, rec: rec}
}

func (r *recorderTestRig) collect(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	return rm
}

func metricByName(t *testing.T, rm metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	t.Fatalf("metric %q not found in collected output", name)
	return metricdata.Metrics{}
}

func sumInt64Point(t *testing.T, m metricdata.Metrics) metricdata.DataPoint[int64] {
	t.Helper()
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("metric %q: expected Sum[int64], got %T", m.Name, m.Data)
	}
	if len(sum.DataPoints) != 1 {
		t.Fatalf("metric %q: expected 1 data point, got %d", m.Name, len(sum.DataPoints))
	}
	return sum.DataPoints[0]
}

func histFloat64Point(t *testing.T, m metricdata.Metrics) metricdata.HistogramDataPoint[float64] {
	t.Helper()
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("metric %q: expected Histogram[float64], got %T", m.Name, m.Data)
	}
	if len(hist.DataPoints) != 1 {
		t.Fatalf("metric %q: expected 1 data point, got %d", m.Name, len(hist.DataPoints))
	}
	return hist.DataPoints[0]
}

func attrStr(t *testing.T, set attribute.Set, key, want string) {
	t.Helper()
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attribute %q missing; have: %v", key, set.Encoded(attribute.DefaultEncoder()))
	}
	if got := v.AsString(); got != want {
		t.Errorf("attribute %q = %q, want %q", key, got, want)
	}
}

func attrInt(t *testing.T, set attribute.Set, key string, want int64) {
	t.Helper()
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		t.Fatalf("attribute %q missing; have: %v", key, set.Encoded(attribute.DefaultEncoder()))
	}
	if got := v.AsInt64(); got != want {
		t.Errorf("attribute %q = %d, want %d", key, got, want)
	}
}

func assertAttrAbsent(t *testing.T, set attribute.Set, key string) {
	t.Helper()
	if _, ok := set.Value(attribute.Key(key)); ok {
		t.Errorf("attribute %q should be absent on this instrument", key)
	}
}

func TestRecorderHTTPRequest(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.HTTPRequest(context.Background(), "/api/foo", 200, 50*time.Millisecond)
	rm := r.collect(t)

	cdp := sumInt64Point(t, metricByName(t, rm, "hollis.http.request.count"))
	if cdp.Value != 1 {
		t.Errorf("count value = %d, want 1", cdp.Value)
	}
	attrStr(t, cdp.Attributes, "app", "test-app")
	attrStr(t, cdp.Attributes, "route", "/api/foo")
	attrInt(t, cdp.Attributes, "status_code", 200)

	hdp := histFloat64Point(t, metricByName(t, rm, "hollis.http.request.duration"))
	if hdp.Count != 1 {
		t.Errorf("duration count = %d, want 1", hdp.Count)
	}
	if hdp.Sum != 50 {
		t.Errorf("duration sum = %v, want 50", hdp.Sum)
	}
	attrStr(t, hdp.Attributes, "app", "test-app")
	attrStr(t, hdp.Attributes, "route", "/api/foo")
	attrInt(t, hdp.Attributes, "status_code", 200)
}

func TestRecorderAgentTurn(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.AgentTurn(context.Background(), "anthropic", "managed", "ok", 250*time.Millisecond)
	rm := r.collect(t)

	hdp := histFloat64Point(t, metricByName(t, rm, "hollis.agent.turn.duration"))
	if hdp.Sum != 250 {
		t.Errorf("duration sum = %v, want 250", hdp.Sum)
	}
	attrStr(t, hdp.Attributes, "app", "test-app")
	attrStr(t, hdp.Attributes, "provider", "anthropic")
	attrStr(t, hdp.Attributes, "runtime_kind", "managed")
	attrStr(t, hdp.Attributes, "result", "ok")
}

// TestRecorderToolCallLabelShapeDivergence pins down the asymmetric label
// set: the count carries result; the duration does not.
func TestRecorderToolCallLabelShapeDivergence(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.ToolCall(context.Background(), "memory_write", "ok", 30*time.Millisecond)
	rm := r.collect(t)

	cdp := sumInt64Point(t, metricByName(t, rm, "hollis.tool.call.count"))
	if cdp.Value != 1 {
		t.Errorf("count = %d, want 1", cdp.Value)
	}
	attrStr(t, cdp.Attributes, "app", "test-app")
	attrStr(t, cdp.Attributes, "tool_name", "memory_write")
	attrStr(t, cdp.Attributes, "result", "ok")

	hdp := histFloat64Point(t, metricByName(t, rm, "hollis.tool.call.duration"))
	attrStr(t, hdp.Attributes, "app", "test-app")
	attrStr(t, hdp.Attributes, "tool_name", "memory_write")
	assertAttrAbsent(t, hdp.Attributes, "result")
}

func TestRecorderMessageLabelShapeDivergence(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.Message(context.Background(), "send", "ok", 10*time.Millisecond)
	rm := r.collect(t)

	cdp := sumInt64Point(t, metricByName(t, rm, "hollis.message.count"))
	attrStr(t, cdp.Attributes, "kind", "send")
	attrStr(t, cdp.Attributes, "result", "ok")

	hdp := histFloat64Point(t, metricByName(t, rm, "hollis.message.duration"))
	attrStr(t, hdp.Attributes, "kind", "send")
	assertAttrAbsent(t, hdp.Attributes, "result")
}

func TestRecorderProviderTokens(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.ProviderTokens(context.Background(), "anthropic", "claude-opus-4-7", 1500, 420)
	rm := r.collect(t)

	in := sumInt64Point(t, metricByName(t, rm, "hollis.provider.tokens.input"))
	if in.Value != 1500 {
		t.Errorf("input tokens = %d, want 1500", in.Value)
	}
	attrStr(t, in.Attributes, "provider", "anthropic")
	attrStr(t, in.Attributes, "model", "claude-opus-4-7")

	out := sumInt64Point(t, metricByName(t, rm, "hollis.provider.tokens.output"))
	if out.Value != 420 {
		t.Errorf("output tokens = %d, want 420", out.Value)
	}
}

func TestRecorderContextTokenBudget(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.ContextTokenBudget(context.Background(), "anthropic", "claude-opus-4-7", 8192)
	rm := r.collect(t)

	hdp := histFloat64Point(t, metricByName(t, rm, "hollis.context.token_budget.used"))
	if hdp.Sum != 8192 {
		t.Errorf("budget sum = %v, want 8192", hdp.Sum)
	}
	attrStr(t, hdp.Attributes, "model", "claude-opus-4-7")
}

// TestRecorderSSEConnectionConverges asserts open+close yields net-zero on
// the UpDownCounter, which is the whole point of using one for active
// connections.
func TestRecorderSSEConnectionConverges(t *testing.T) {
	ctx := context.Background()
	r := newRecorderTestRig(t)
	r.rec.SSEConnectionOpened(ctx, "agent_events")
	r.rec.SSEConnectionOpened(ctx, "agent_events")
	r.rec.SSEConnectionClosed(ctx, "agent_events")
	rm := r.collect(t)

	dp := sumInt64Point(t, metricByName(t, rm, "hollis.sse.active_connections"))
	if dp.Value != 1 {
		t.Errorf("active connections = %d, want 1 (2 opened, 1 closed)", dp.Value)
	}
	attrStr(t, dp.Attributes, "stream_type", "agent_events")
}

func TestRecorderSSEReconnect(t *testing.T) {
	r := newRecorderTestRig(t)
	r.rec.SSEReconnect(context.Background(), "agent_events")
	rm := r.collect(t)

	dp := sumInt64Point(t, metricByName(t, rm, "hollis.sse.reconnects"))
	if dp.Value != 1 {
		t.Errorf("reconnects = %d, want 1", dp.Value)
	}
	attrStr(t, dp.Attributes, "stream_type", "agent_events")
}

// TestRecorderQueueDepthSignedDelta exercises the signed-delta contract:
// positive enqueue, negative dequeue, net difference observed.
func TestRecorderQueueDepthSignedDelta(t *testing.T) {
	ctx := context.Background()
	r := newRecorderTestRig(t)
	r.rec.QueueDepth(ctx, "broker_inbox", 5)
	r.rec.QueueDepth(ctx, "broker_inbox", -2)
	rm := r.collect(t)

	dp := sumInt64Point(t, metricByName(t, rm, "hollis.queue.depth"))
	if dp.Value != 3 {
		t.Errorf("queue depth = %d, want 3 (+5 -2)", dp.Value)
	}
	attrStr(t, dp.Attributes, "queue_name", "broker_inbox")
}

// TestNewRecorderWrapsExistingMetrics confirms the two-step constructor
// works when callers want to share one Metrics across multiple recorders.
func TestNewRecorderWrapsExistingMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := RegisterMetrics(provider.Meter("shared"))
	if err != nil {
		t.Fatalf("RegisterMetrics() error = %v", err)
	}

	r1 := NewRecorder(m, "app-1")
	r2 := NewRecorder(m, "app-2")
	if r1.App() != "app-1" || r2.App() != "app-2" {
		t.Fatalf("App() = %q / %q, want app-1 / app-2", r1.App(), r2.App())
	}
	if r1.Metrics() != r2.Metrics() {
		t.Errorf("Metrics() should return the same underlying handle for both recorders")
	}
}
