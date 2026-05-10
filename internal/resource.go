// Package internal contains shared helpers for the go-otel module and is
// not part of the public API.
package internal

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewResource builds an OTel Resource with the standard service-identity
// attributes (service.name, service.version, deployment.environment, and
// optionally service.namespace when non-empty).
//
// Plain attribute keys are used to avoid semconv schema-URL conflicts with
// resource.Default().
func NewResource(serviceName, serviceVersion, serviceNamespace, environment string) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", serviceName),
		attribute.String("service.version", serviceVersion),
		attribute.String("deployment.environment", environment),
	}
	if serviceNamespace != "" {
		attrs = append(attrs, attribute.String("service.namespace", serviceNamespace))
	}
	return resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attrs...),
	)
}
