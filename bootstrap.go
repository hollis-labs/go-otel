package hotel

import (
	"context"
	"os"
	"strings"
	"time"
)

// InitOrWarn calls Init and, on failure, reports the error through logf and
// returns a safe no-op shutdown — so callers can always `defer shutdown()`
// unconditionally, with no if/else around Init's error.
//
// On success, the returned shutdown is Init's own shutdown already bound to
// ShutdownWithTimeout(shutdown, timeout): callers get a bounded-shutdown
// guarantee (no risk of an exporter flush hanging process exit) without asking
// for it. Pass the same timeout you would have passed to ShutdownWithTimeout
// yourself; 5*time.Second matches every existing portfolio call site that
// already bounds its shutdown. A shutdown error is reported through logf, not
// returned.
//
// logf matches log.Printf's signature, so the stdlib logger can be passed
// directly: hotel.InitOrWarn(ctx, log.Printf, 5*time.Second, opts...).
//
// InitOrWarn is deliberately polarity-agnostic: it does not decide whether
// telemetry is enabled. An app that has its own enable/disable switch keeps
// that gate and calls InitOrWarn inside it.
func InitOrWarn(ctx context.Context, logf func(format string, args ...any), timeout time.Duration, opts ...Option) (shutdown func()) {
	sd, err := Init(ctx, opts...)
	if err != nil {
		logf("warning: OTel init failed: %v", err)
		return func() {}
	}
	return func() {
		if err := ShutdownWithTimeout(sd, timeout); err != nil {
			logf("warning: OTel shutdown failed: %v", err)
		}
	}
}

// EnvironmentFromEnv resolves the deployment-environment tag (dev / staging /
// uat / prod ...) for WithEnvironment. HOLLIS_ENV (the portfolio-wide override)
// wins first, then appEnvVar (an app-specific override, for example
// "TORQUE_ENV"; pass "" to skip this tier), then fallback. Values are trimmed;
// an empty or whitespace-only value counts as unset.
func EnvironmentFromEnv(appEnvVar, fallback string) string {
	if v := trimmedEnv("HOLLIS_ENV"); v != "" {
		return v
	}
	if appEnvVar != "" {
		if v := trimmedEnv(appEnvVar); v != "" {
			return v
		}
	}
	return fallback
}

// EnabledFromEnv reports whether telemetry should be initialized, under the
// opt-in policy every hollis-labs app follows: disabled unless explicitly
// turned on. HOLLIS_OTEL_ENABLED (portfolio-wide) is consulted first, then
// appEnvVar (an app-specific override, for example "MYAPP_OTEL_ENABLED"; pass
// "" to skip this tier). Both accept "1", "true", "yes" and "on",
// case-insensitively, after trimming whitespace.
//
// Unlike EnvironmentFromEnv there is no fallback parameter: the floor is
// always "disabled". Unset, empty, unparseable and falsey values all mean
// disabled, and a falsey value in one tier does not veto a truthy value in
// the other: either tier can turn telemetry on, neither can force it off.
//
// The result only reports the gate. Init and InitOrWarn do not consult it;
// gating stays the caller's job:
//
//	if hotel.EnabledFromEnv("MYAPP_OTEL_ENABLED") {
//		defer hotel.InitOrWarn(ctx, log.Printf, 5*time.Second, opts...)()
//	}
func EnabledFromEnv(appEnvVar string) bool {
	if truthy(trimmedEnv("HOLLIS_OTEL_ENABLED")) {
		return true
	}
	if appEnvVar != "" && truthy(trimmedEnv(appEnvVar)) {
		return true
	}
	return false
}

func truthy(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func trimmedEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
