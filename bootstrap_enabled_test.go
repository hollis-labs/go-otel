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
	// Presence-based precedence: a non-blank HOLLIS_OTEL_ENABLED decides alone
	// (truthy enables, anything else disables) and the app var is ignored; a
	// unset/blank global defers to the app var. Disabled is the floor.
	tests := []struct {
		name      string
		global    *string // nil = unset
		app       *string
		appEnvVar string
		want      bool
	}{
		// Global unset: app tier decides.
		{"global unset app unset", nil, nil, testAppVar, false},
		{"global unset app 1", nil, sp("1"), testAppVar, true},
		{"global unset app true", nil, sp("true"), testAppVar, true},
		{"global unset app yes", nil, sp("yes"), testAppVar, true},
		{"global unset app on", nil, sp("on"), testAppVar, true},
		{"global unset app TRUE", nil, sp("TRUE"), testAppVar, true},
		{"global unset app Yes", nil, sp("Yes"), testAppVar, true},
		{"global unset app ON", nil, sp("ON"), testAppVar, true},
		{"global unset app padded", nil, sp("  on \n"), testAppVar, true},
		{"global unset app false", nil, sp("false"), testAppVar, false},
		{"global unset app 0", nil, sp("0"), testAppVar, false},
		{"global unset app garbage", nil, sp("banana"), testAppVar, false},
		{"global unset app blank", nil, sp("  "), testAppVar, false},
		{"global unset app y/t not truthy", nil, sp("y"), testAppVar, false},
		// Global blank / whitespace-only counts as unset: falls through.
		{"global empty app true", sp(""), sp("true"), testAppVar, true},
		{"global empty app false", sp(""), sp("false"), testAppVar, false},
		{"global empty app unset", sp(""), nil, testAppVar, false},
		{"global whitespace app true", sp(" \t "), sp("true"), testAppVar, true},
		{"global whitespace app garbage", sp("   "), sp("banana"), testAppVar, false},
		{"global whitespace app unset", sp("   "), nil, testAppVar, false},
		// Global truthy decides: enabled regardless of app.
		{"global 1 app unset", sp("1"), nil, testAppVar, true},
		{"global true app false", sp("true"), sp("false"), testAppVar, true},
		{"global yes app garbage", sp("yes"), sp("banana"), testAppVar, true},
		{"global on app true", sp("on"), sp("true"), testAppVar, true},
		{"global TRUE app unset", sp("TRUE"), nil, testAppVar, true},
		{"global Yes padded app 0", sp("  Yes "), sp("0"), testAppVar, true},
		// Global falsey / garbage decides: disabled, app ignored (veto).
		{"global false app true", sp("false"), sp("true"), testAppVar, false},
		{"global 0 app true", sp("0"), sp("1"), testAppVar, false},
		{"global off app on", sp("off"), sp("on"), testAppVar, false},
		{"global FALSE app yes", sp("FALSE"), sp("yes"), testAppVar, false},
		{"global false padded app true", sp(" false "), sp("true"), testAppVar, false},
		{"global false app unset", sp("false"), nil, testAppVar, false},
		{"global garbage app true", sp("banana"), sp("true"), testAppVar, false},
		{"global 2 app on", sp("2"), sp("on"), testAppVar, false},
		{"global y app true", sp("y"), sp("true"), testAppVar, false},
		{"global garbage app unset", sp("banana"), nil, testAppVar, false},
		// appEnvVar == "" skips the app tier.
		{"empty appEnvVar global unset", nil, sp("1"), "", false},
		{"empty appEnvVar global blank", sp(" "), sp("1"), "", false},
		{"empty appEnvVar global true", sp("1"), sp("0"), "", true},
		{"empty appEnvVar global false", sp("false"), sp("1"), "", false},
		{"empty appEnvVar global garbage", sp("banana"), nil, "", false},
		{"unset named appEnvVar", nil, sp("1"), "SOME_OTHER_UNSET_VAR_XYZ", false},
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
// from unset, empty or unparseable input, in either tier and any combination:
// opt-in is the only way on.
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
