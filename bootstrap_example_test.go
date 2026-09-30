package hotel_test

import (
	"context"
	"fmt"
	"log"
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
