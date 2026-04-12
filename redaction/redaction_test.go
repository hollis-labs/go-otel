package redaction

import "testing"

func TestShouldRedactDefaults(t *testing.T) {
	t.Setenv("FE_OTEL_REDACT_PROMPTS", "")

	if !ShouldRedact("gen_ai.content.prompt") {
		t.Fatal("prompt content should be redacted by default")
	}
	if !ShouldRedact("gen_ai.content.completion") {
		t.Fatal("completion content should be redacted by default")
	}
	if ShouldRedact("gen_ai.request.model") {
		t.Fatal("non-denylisted keys should not be redacted")
	}
}

func TestShouldRedactCanBeDisabled(t *testing.T) {
	t.Setenv("FE_OTEL_REDACT_PROMPTS", "false")

	if ShouldRedact("gen_ai.content.prompt") {
		t.Fatal("redaction should be disabled when FE_OTEL_REDACT_PROMPTS=false")
	}
}

func TestDenylist(t *testing.T) {
	got := Denylist()
	want := map[string]bool{
		"gen_ai.content.prompt":     false,
		"gen_ai.content.completion": false,
	}
	if len(got) != len(want) {
		t.Fatalf("denylist length = %d, want %d", len(got), len(want))
	}
	for _, key := range got {
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected denylist key %q", key)
		}
	}
}
