// Package internal contains shared helpers for the otel module.
package internal

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewResource builds an OTel Resource with standard Fragments Engine attributes.
// Uses plain attribute keys to avoid semconv schema URL conflicts with resource.Default().
func NewResource(serviceName, serviceVersion, environment string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion),
			attribute.String("deployment.environment", environment),
			attribute.String("service.namespace", "fragments-engine"),
		),
	)
}
