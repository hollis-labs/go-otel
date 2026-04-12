// Command hello is a minimal example that demonstrates otel instrumentation
// using a stdout exporter (no collector required).
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	feotel "github.com/hollis-labs/go-otel"
	"github.com/hollis-labs/go-otel/genai"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	// For this example we use stdout exporter instead of OTLP.
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Demonstrate that Init works (would connect to OTLP in production).
	_ = feotel.WithServiceName("hello-fe")

	// Create a root span.
	ctx, rootSpan := feotel.StartSpan(ctx, "hello.request")
	defer rootSpan.End()

	// Create a GenAI model call child span.
	ctx, modelSpan := genai.ModelCallSpan(ctx, "claude-opus-4-6", "chat")
	time.Sleep(50 * time.Millisecond)
	genai.RecordTokenUsage(modelSpan, 150, 42)
	genai.RecordModelLatency(ctx, "claude-opus-4-6", 50*time.Millisecond)
	modelSpan.End()

	fmt.Println("hello-fe example complete")
}
