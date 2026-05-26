// Package internal contains shared helpers for the go-otel module and is
// not part of the public API.
package internal

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewResource builds an OTel Resource with the standard service-identity
// attributes (service.name, service.version, deployment.environment, and
// optionally service.namespace when non-empty), layered on top of any
// caller-supplied resource.Options (typically detectors like
// resource.WithHost(), resource.WithProcess(), resource.WithContainer()).
//
// Precedence (highest → lowest):
//  1. Service-identity attributes set here (service.name, etc.).
//  2. Caller-supplied resource.Options.
//  3. resource.Default() — SDK info, OTEL_RESOURCE_ATTRIBUTES, default
//     service name.
//
// Plain attribute keys are used to avoid semconv schema-URL conflicts with
// resource.Default().
func NewResource(ctx context.Context, serviceName, serviceVersion, serviceNamespace, environment string, extras ...resource.Option) (*resource.Resource, error) {
	detected, err := resource.New(ctx, extras...)
	if err != nil {
		return nil, err
	}
	withDefaults, err := resource.Merge(resource.Default(), detected)
	if err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{
		attribute.String("service.name", serviceName),
		attribute.String("service.version", serviceVersion),
		attribute.String("deployment.environment", environment),
	}
	if serviceNamespace != "" {
		attrs = append(attrs, attribute.String("service.namespace", serviceNamespace))
	}
	return resource.Merge(withDefaults, resource.NewSchemaless(attrs...))
}
