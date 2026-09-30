package propagation

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	otelpropagation "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	testTraceID = "0102030405060708090a0b0c0d0e0f10"
	testSpanID  = "1112131415161718"
)

func useTraceContextPropagator(t *testing.T) {
	t.Helper()
	prev := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{}, otelpropagation.Baggage{},
	))
}

func spanCtx(t *testing.T, flags trace.TraceFlags, tracestate string) context.Context {
	t.Helper()
	tid, _ := trace.TraceIDFromHex(testTraceID)
	sid, _ := trace.SpanIDFromHex(testSpanID)
	cfg := trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: flags}
	if tracestate != "" {
		ts, err := trace.ParseTraceState(tracestate)
		if err != nil {
			t.Fatal(err)
		}
		cfg.TraceState = ts
	}
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(cfg))
}

func TestInjectAndExtractMCPMeta(t *testing.T) {
	useTraceContextPropagator(t)
	for _, tc := range []struct {
		name  string
		flags trace.TraceFlags
		state string
		tp    string
	}{
		{"sampled", trace.FlagsSampled, "", "00-" + testTraceID + "-" + testSpanID + "-01"},
		{"unsampled", 0, "", "00-" + testTraceID + "-" + testSpanID + "-00"},
		{"tracestate", trace.FlagsSampled, "vendor=abc,other=xyz", "00-" + testTraceID + "-" + testSpanID + "-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := InjectMCPMeta(spanCtx(t, tc.flags, tc.state), nil)
			if meta["_traceparent"] != tc.tp {
				t.Fatalf("_traceparent = %v, want %v", meta["_traceparent"], tc.tp)
			}
			got := trace.SpanContextFromContext(ExtractMCPMeta(context.Background(), meta))
			if got.TraceID().String() != testTraceID || got.SpanID().String() != testSpanID {
				t.Fatalf("ids = %s/%s", got.TraceID(), got.SpanID())
			}
			if got.IsSampled() != (tc.flags == trace.FlagsSampled) {
				t.Errorf("sampled = %v", got.IsSampled())
			}
			if got.TraceState().String() != tc.state {
				t.Errorf("tracestate = %q, want %q", got.TraceState().String(), tc.state)
			}
			if !got.IsRemote() {
				t.Error("extracted span context should be remote")
			}
		})
	}
}

func TestInjectMCPMetaWireKeys(t *testing.T) {
	meta := InjectMCPMeta(spanCtx(t, trace.FlagsSampled, "vendor=abc"), nil)
	want := map[string]any{
		"_traceparent": "00-" + testTraceID + "-" + testSpanID + "-01",
		"_tracestate":  "vendor=abc",
	}
	if !reflect.DeepEqual(meta, want) {
		t.Fatalf("meta = %v, want %v", meta, want)
	}
}

func TestInjectMCPMetaInvalidSpanReturnsMetaUnchanged(t *testing.T) {
	if got := InjectMCPMeta(context.Background(), nil); got != nil {
		t.Errorf("nil meta, no span: got %v, want nil", got)
	}
	in := map[string]any{"a": 1}
	got := InjectMCPMeta(context.Background(), in)
	if !reflect.DeepEqual(got, map[string]any{"a": 1}) || len(in) != 1 {
		t.Errorf("got %v, in %v", got, in)
	}
}

func TestInjectMCPMetaNilAndEmptyMetaUsable(t *testing.T) {
	ctx := spanCtx(t, trace.FlagsSampled, "")
	for _, in := range []map[string]any{nil, {}} {
		got := InjectMCPMeta(ctx, in)
		if got == nil || got["_traceparent"] == nil {
			t.Fatalf("got %v", got)
		}
		got["x"] = 1 // must be writable
	}
}

func TestInjectMCPMetaDoesNotMutateCallerMap(t *testing.T) {
	in := map[string]any{"hadron/idempotencyKey": "k1", "_tracestate": "stale=1"}
	out := InjectMCPMeta(spanCtx(t, trace.FlagsSampled, ""), in)
	if !reflect.DeepEqual(in, map[string]any{"hadron/idempotencyKey": "k1", "_tracestate": "stale=1"}) {
		t.Fatalf("caller map mutated: %v", in)
	}
	if _, ok := out["_tracestate"]; ok {
		t.Errorf("stale _tracestate not dropped: %v", out)
	}
	out["later"] = true
	if _, ok := in["later"]; ok {
		t.Error("returned map aliases caller map")
	}
}

func TestMCPMetaPreservesUnrelatedKeys(t *testing.T) {
	useTraceContextPropagator(t)
	in := map[string]any{"hadron/idempotencyKey": "abc", "progressToken": 7}
	out := InjectMCPMeta(spanCtx(t, trace.FlagsSampled, ""), in)
	if out["hadron/idempotencyKey"] != "abc" || out["progressToken"] != 7 {
		t.Fatalf("unrelated keys lost: %v", out)
	}
	_ = ExtractMCPMeta(context.Background(), out)
	if out["hadron/idempotencyKey"] != "abc" || len(out) != 3 {
		t.Fatalf("extract altered meta: %v", out)
	}
}

func TestExtractMCPMetaNilEmptyAndInvalid(t *testing.T) {
	useTraceContextPropagator(t)
	for name, meta := range map[string]map[string]any{
		"nil":            nil,
		"empty":          {},
		"non-string":     {"_traceparent": 42, "_tracestate": []string{"x"}},
		"garbage":        {"_traceparent": "not-a-traceparent"},
		"zero ids":       {"_traceparent": "00-00000000000000000000000000000000-0000000000000000-01"},
		"unrelated only": {"hadron/idempotencyKey": "k"},
		"bare keys":      {"traceparent": "00-" + testTraceID + "-" + testSpanID + "-01"},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ctxKey{}, "s")
			got := ExtractMCPMeta(ctx, meta)
			if trace.SpanContextFromContext(got).IsValid() {
				t.Error("unexpected span context")
			}
			if got.Value(ctxKey{}) != "s" {
				t.Error("caller ctx lost")
			}
		})
	}
}

func TestExtractMCPMetaPreservesCallerContext(t *testing.T) {
	useTraceContextPropagator(t)
	meta := InjectMCPMeta(spanCtx(t, trace.FlagsSampled, ""), nil)

	deadline := time.Now().Add(time.Hour)
	parent, cancelDeadline := context.WithDeadline(context.WithValue(context.Background(), ctxKey{}, "sentinel"), deadline)
	defer cancelDeadline()
	ctx, cancel := context.WithCancel(parent)

	got := ExtractMCPMeta(ctx, meta)
	if v, _ := got.Value(ctxKey{}).(string); v != "sentinel" {
		t.Errorf("value lost: %q", v)
	}
	if d, ok := got.Deadline(); !ok || !d.Equal(deadline) {
		t.Errorf("deadline lost: %v %v", d, ok)
	}
	if trace.SpanContextFromContext(got).TraceID().String() != testTraceID {
		t.Error("trace context not extracted")
	}
	cancel()
	select {
	case <-got.Done():
	case <-time.After(time.Second):
		t.Fatal("cancellation not propagated")
	}
}

func TestExtractMCPMetaWithoutPropagatorKeepsCallerContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator())

	ctx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	got := ExtractMCPMeta(ctx, map[string]any{"_traceparent": "00-" + testTraceID + "-" + testSpanID + "-01"})
	if v, _ := got.Value(ctxKey{}).(string); v != "sentinel" {
		t.Errorf("value lost: %q", v)
	}
	if trace.SpanContextFromContext(got).IsValid() {
		t.Error("trace context appeared with no propagator")
	}
}

// _meta and arguments are independent carriers: trace context in a tool's
// arguments is invisible to ExtractMCPMeta (no fallback), and injecting into
// one map never affects a separately passed other.
func TestMetaAndArgumentsAreIndependent(t *testing.T) {
	useTraceContextPropagator(t)
	ctx := spanCtx(t, trace.FlagsSampled, "")

	args := map[string]any{"q": "x"}
	meta := map[string]any{"progressToken": 1}

	meta = InjectMCPMeta(ctx, meta)
	if !reflect.DeepEqual(args, map[string]any{"q": "x"}) {
		t.Fatalf("InjectMCPMeta touched arguments: %v", args)
	}
	args = InjectMCP(ctx, args) // deprecated arguments path, unchanged
	if len(meta) != 2 {
		t.Fatalf("InjectMCP changed a separate _meta map: %v", meta)
	}

	// Trace context only in arguments: ExtractMCPMeta over empty _meta finds none.
	got := ExtractMCPMeta(context.Background(), map[string]any{"progressToken": 1})
	if trace.SpanContextFromContext(got).IsValid() {
		t.Fatal("ExtractMCPMeta recovered trace context without _meta (arguments fallback?)")
	}
	if args["_traceparent"] == nil {
		t.Fatal("setup: arguments carry no trace context")
	}
}

func TestMCPMetaConcurrentUse(t *testing.T) {
	useTraceContextPropagator(t)
	ctx := spanCtx(t, trace.FlagsSampled, "vendor=abc")
	shared := map[string]any{"hadron/idempotencyKey": "k"}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				out := InjectMCPMeta(ctx, shared)
				out[fmt.Sprint("g", i)] = j // private copy: safe to write
				got := trace.SpanContextFromContext(ExtractMCPMeta(context.Background(), out))
				if got.TraceID().String() != testTraceID {
					t.Error("bad round trip")
					return
				}
			}
		}(i)
	}
	wg.Wait()
	if len(shared) != 1 {
		t.Fatalf("shared map mutated: %v", shared)
	}
}

func FuzzExtractMCPMeta(f *testing.F) {
	otel.SetTextMapPropagator(otelpropagation.NewCompositeTextMapPropagator(
		otelpropagation.TraceContext{}, otelpropagation.Baggage{},
	))
	f.Add("00-"+testTraceID+"-"+testSpanID+"-01", "vendor=abc")
	f.Add("", "")
	f.Add("00-zz-yy-01", "a=b,,=,")
	f.Add("ff-00000000000000000000000000000000-0000000000000000-ff", "k=v\x00")
	f.Fuzz(func(t *testing.T, tp, ts string) {
		for _, meta := range []map[string]any{
			{"_traceparent": tp, "_tracestate": ts},
			{"_traceparent": tp},
			{"_traceparent": []byte(tp), "_tracestate": nil},
		} {
			ctx := ExtractMCPMeta(context.Background(), meta)
			if ctx == nil {
				t.Fatal("nil ctx")
			}
			_ = trace.SpanContextFromContext(ctx)
		}
	})
}
