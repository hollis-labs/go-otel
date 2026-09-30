package hotel

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

const testAppVar = "TESTAPP_OTEL_ENABLED"

func TestEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		global    *string // nil = unset
		app       *string
		appEnvVar string
		want      bool
	}{
		{"nothing set", nil, nil, testAppVar, false},
		{"global alone truthy", sp("1"), nil, testAppVar, true},
		{"global alone true", sp("true"), nil, "", true},
		{"app alone 1", nil, sp("1"), testAppVar, true},
		{"app alone true", nil, sp("true"), testAppVar, true},
		{"app alone yes", nil, sp("yes"), testAppVar, true},
		{"app alone on", nil, sp("on"), testAppVar, true},
		{"app alone upper TRUE", nil, sp("TRUE"), testAppVar, true},
		{"app alone mixed Yes", nil, sp("Yes"), testAppVar, true},
		{"app alone ON", nil, sp("ON"), testAppVar, true},
		{"global upper case", sp("True"), nil, testAppVar, true},
		{"app falsey 0", nil, sp("0"), testAppVar, false},
		{"app falsey false", nil, sp("false"), testAppVar, false},
		{"app falsey no", nil, sp("no"), testAppVar, false},
		{"app falsey off", nil, sp("off"), testAppVar, false},
		{"app empty", nil, sp(""), testAppVar, false},
		{"app blank", nil, sp("   "), testAppVar, false},
		{"global falsey", sp("false"), nil, testAppVar, false},
		{"global empty", sp(""), nil, testAppVar, false},
		{"garbage global", sp("banana"), nil, testAppVar, false},
		{"garbage app", nil, sp("banana"), testAppVar, false},
		{"garbage both", sp("2"), sp("enabled"), testAppVar, false},
		{"y and t are not truthy", sp("y"), sp("t"), testAppVar, false},
		{"both truthy", sp("1"), sp("1"), testAppVar, true},
		{"global true app false", sp("true"), sp("false"), testAppVar, true},
		// Either tier can enable; a falsey value never vetoes the other tier.
		{"global false app true", sp("false"), sp("true"), testAppVar, true},
		{"global garbage app true", sp("banana"), sp("on"), testAppVar, true},
		{"empty appEnvVar skips app tier", nil, sp("1"), "", false},
		{"empty appEnvVar still honors global", sp("1"), sp("0"), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setOrUnset(t, "HOLLIS_OTEL_ENABLED", tt.global)
			setOrUnset(t, testAppVar, tt.app)
			if got := EnabledFromEnv(tt.appEnvVar); got != tt.want {
				t.Fatalf("EnabledFromEnv(%q) = %v, want %v", tt.appEnvVar, got, tt.want)
			}
		})
	}
}

func TestEnabledFromEnv_TrimsWhitespace(t *testing.T) {
	for _, v := range []string{" 1", "true ", "\tyes\n", "  ON  "} {
		t.Run(fmt.Sprintf("app %q", v), func(t *testing.T) {
			setOrUnset(t, "HOLLIS_OTEL_ENABLED", nil)
			setOrUnset(t, testAppVar, &v)
			if !EnabledFromEnv(testAppVar) {
				t.Fatalf("app value %q should count as enabled", v)
			}
		})
		t.Run(fmt.Sprintf("global %q", v), func(t *testing.T) {
			setOrUnset(t, "HOLLIS_OTEL_ENABLED", &v)
			setOrUnset(t, testAppVar, nil)
			if !EnabledFromEnv(testAppVar) {
				t.Fatalf("global value %q should count as enabled", v)
			}
		})
	}
}

// TestEnabledFromEnv_DisabledIsTheFloor proves there is no path to "enabled"
// from unset, empty or unparseable input: opt-in is the only way on.
func TestEnabledFromEnv_DisabledIsTheFloor(t *testing.T) {
	junk := []string{"", " ", "0", "false", "no", "off", "banana", "2", "-1", "truee", "1.0", "enabled", "null", "y", "t"}
	for _, g := range junk {
		for _, a := range junk {
			setOrUnset(t, "HOLLIS_OTEL_ENABLED", &g)
			setOrUnset(t, testAppVar, &a)
			if EnabledFromEnv(testAppVar) {
				t.Fatalf("global=%q app=%q enabled; disabled must be the floor", g, a)
			}
		}
	}
	setOrUnset(t, "HOLLIS_OTEL_ENABLED", nil)
	setOrUnset(t, testAppVar, nil)
	if EnabledFromEnv(testAppVar) || EnabledFromEnv("") || EnabledFromEnv("SOME_UNSET_VAR_XYZ") {
		t.Fatal("unset environment must be disabled")
	}
}

// TestEnabledFromEnv_ConcurrentCalls has no package-level mutable state to
// race on; run under -race this pins that.
func TestEnabledFromEnv_ConcurrentCalls(t *testing.T) {
	t.Setenv("HOLLIS_OTEL_ENABLED", "")
	t.Setenv(testAppVar, "on")
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if !EnabledFromEnv(testAppVar) {
					t.Error("expected enabled")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestTruthy(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", "on", "TRUE", "Yes", "oN"} {
		if !truthy(v) {
			t.Errorf("truthy(%q) = false", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no", "off", "x", strings.Repeat("1", 2)} {
		if truthy(v) {
			t.Errorf("truthy(%q) = true", v)
		}
	}
}

func sp(s string) *string { return &s }

// setOrUnset sets key for the test, or guarantees it is unset (restoring the
// prior value at cleanup) when v is nil.
func setOrUnset(t *testing.T, key string, v *string) {
	t.Helper()
	t.Setenv(key, "") // registers restore of the original value
	if v != nil {
		t.Setenv(key, *v)
		return
	}
	if err := unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func unsetenv(key string) error { return os.Unsetenv(key) }
