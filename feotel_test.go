package feotel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInitAndShutdownWithLocalOTLPHTTPServer(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	endpoint := strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdown, err := Init(ctx,
		WithServiceName("test-service"),
		WithServiceVersion("1.2.3"),
		WithEnvironment("test"),
		WithOTLPEndpoint(endpoint),
	)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	spanCtx, span := StartSpan(ctx, "test.span")
	span.End()
	_ = spanCtx

	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}
