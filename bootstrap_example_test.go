package hotel_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	hotel "github.com/hollis-labs/go-otel"
)

func ExampleInitOrWarn() {
	shutdown := hotel.InitOrWarn(context.Background(), log.Printf, 5*time.Second,
		hotel.WithServiceName("myapp"),
		hotel.WithEnvironment(hotel.EnvironmentFromEnv("MYAPP_ENV", "development")),
	)
	defer shutdown() // always safe: a no-op if Init failed, bounded to 5s otherwise
}

func ExampleEnvironmentFromEnv() {
	// HOLLIS_ENV wins, then MYAPP_ENV, then the fallback.
	fmt.Println(hotel.EnvironmentFromEnv("MYAPP_ENV", "development") != "")
	// Output: true
}

func ExampleEnabledFromEnv() {
	// Opt-in: disabled unless HOLLIS_OTEL_ENABLED or MYAPP_OTEL_ENABLED is
	// set to 1/true/yes/on. Nothing set means disabled.
	os.Unsetenv("HOLLIS_OTEL_ENABLED")
	os.Unsetenv("MYAPP_OTEL_ENABLED")
	fmt.Println(hotel.EnabledFromEnv("MYAPP_OTEL_ENABLED"))

	os.Setenv("MYAPP_OTEL_ENABLED", "yes")
	defer os.Unsetenv("MYAPP_OTEL_ENABLED")
	fmt.Println(hotel.EnabledFromEnv("MYAPP_OTEL_ENABLED"))
	// Output:
	// false
	// true
}
