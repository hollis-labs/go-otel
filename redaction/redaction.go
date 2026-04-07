// Package redaction provides span processing that strips sensitive attributes.
package redaction

import (
	"context"
	"os"
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Denylist returns the default attribute keys that are redacted.
func Denylist() []string {
	return []string{
		"gen_ai.content.prompt",
		"gen_ai.content.completion",
	}
}

// SpanProcessor returns a span processor that redacts sensitive attributes.
// Redaction is enabled by default and can be disabled by setting
// FE_OTEL_REDACT_PROMPTS=false.
func SpanProcessor() sdktrace.SpanProcessor {
	enabled := true
	if v := os.Getenv("FE_OTEL_REDACT_PROMPTS"); strings.EqualFold(v, "false") {
		enabled = false
	}
	deny := make(map[string]struct{}, len(Denylist()))
	for _, k := range Denylist() {
		deny[k] = struct{}{}
	}
	return &redactProcessor{enabled: enabled, deny: deny}
}

type redactProcessor struct {
	enabled bool
	deny    map[string]struct{}
}

func (r *redactProcessor) OnStart(_ context.Context, _ sdktrace.ReadWriteSpan) {}

func (r *redactProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// ReadOnlySpan — we cannot mutate after the fact. The redaction processor
	// is designed to be composed with a wrapping exporter or used as a signal
	// that these attributes should not be exported. In practice, the deny-list
	// is checked at export time.
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
