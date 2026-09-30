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

func trimmedEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
