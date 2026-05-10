// Command hello is a minimal example that demonstrates the go-otel
// instrumentation surface using a stdout exporter (no collector required).
//
// It creates a root span via feotel.StartSpan, opens a child span using the
// genai sub-package's ModelCallSpan helper, records token usage and a model
// latency histogram, and prints both spans as JSON via the stdout exporter.
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

	// In a real service you would call feotel.Init here to install the
	// OTLP HTTP exporter and propagators; this example wires up a stdout
	// TracerProvider directly so it can run with no collector.
	_ = feotel.WithServiceName("hello")

	// Create a root span.
	ctx, rootSpan := feotel.StartSpan(ctx, "hello.request")
	defer rootSpan.End()

	// Create a GenAI model call child span.
	ctx, modelSpan := genai.ModelCallSpan(ctx, "claude-opus-4-6", "chat")
	time.Sleep(50 * time.Millisecond)
	genai.RecordTokenUsage(modelSpan, 150, 42)
	genai.RecordModelLatency(ctx, "claude-opus-4-6", 50*time.Millisecond)
	modelSpan.End()

	fmt.Println("hello example complete")
}
