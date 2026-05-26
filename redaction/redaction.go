// Package redaction provides denylist helpers for sensitive GenAI attributes.
package redaction

import (
	"context"
	"os"
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Denylist returns the default attribute keys that should be redacted by
// downstream exporters or span wrappers.
func Denylist() []string {
	return []string{
		"gen_ai.content.prompt",
		"gen_ai.content.completion",
	}
}

// ShouldRedact reports whether the provided attribute key should be removed
// before export.
func ShouldRedact(key string) bool {
	return shouldRedact(os.Getenv("HOLLIS_OTEL_REDACT_PROMPTS"), key)
}

// SpanProcessor returns a processor shell that preserves the denylist
// decision. It does not mutate spans because sdktrace.ReadOnlySpan is
// immutable; callers should consult ShouldRedact before export.
func SpanProcessor() sdktrace.SpanProcessor {
	return &redactProcessor{
		enabled: shouldRedactEnabled(os.Getenv("HOLLIS_OTEL_REDACT_PROMPTS")),
		deny:    denylistSet(),
	}
}

type redactProcessor struct {
	enabled bool
	deny    map[string]struct{}
}

func (r *redactProcessor) OnStart(_ context.Context, _ sdktrace.ReadWriteSpan) {}

func (r *redactProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// ReadOnlySpan is immutable; enforcement must happen in a wrapping exporter.
	_ = s
}

func (r *redactProcessor) Shutdown(_ context.Context) error   { return nil }
func (r *redactProcessor) ForceFlush(_ context.Context) error { return nil }

// ShouldRedact reports whether the given attribute key is on the denylist
// and redaction is enabled.
func (r *redactProcessor) ShouldRedact(key string) bool {
	if !r.enabled {
		return false
	}
	_, found := r.deny[key]
	return found
}

func shouldRedactEnabled(v string) bool {
	return !strings.EqualFold(v, "false")
}

func shouldRedact(envValue, key string) bool {
	if !shouldRedactEnabled(envValue) {
		return false
	}
	deny := denylistSet()
	_, found := deny[key]
	return found
}

func denylistSet() map[string]struct{} {
	deny := make(map[string]struct{}, len(Denylist()))
	for _, k := range Denylist() {
		deny[k] = struct{}{}
	}
	return deny
}
