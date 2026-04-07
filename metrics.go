package feotel

import (
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the standard metric instruments.
type Metrics struct {
	RequestCount   metric.Int64Counter
	RequestLatency metric.Float64Histogram
	ErrorCount     metric.Int64Counter
}

// RegisterMetrics creates and returns the standard metric instruments.
func RegisterMetrics(meter metric.Meter) (*Metrics, error) {
	reqCount, err := meter.Int64Counter("fe.request.count",
		metric.WithDescription("Total number of requests handled"),
	)
	if err != nil {
		return nil, err
	}

	reqLatency, err := meter.Float64Histogram("fe.request.latency",
		metric.WithDescription("Request latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	errCount, err := meter.Int64Counter("fe.error.count",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		RequestCount:   reqCount,
		RequestLatency: reqLatency,
		ErrorCount:     errCount,
	}, nil
}
