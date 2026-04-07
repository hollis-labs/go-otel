---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Workflows

## Local development

1. Clone or navigate to `~/Projects-apps/tiamat-otel/`
2. Run `go mod tidy` to ensure dependencies are resolved
3. Edit source files in root or sub-packages
4. Consumer projects reference this module via `replace github.com/hollis-labs/otel => ../tiamat-otel` in their `go.mod`

## Build

```bash
cd ~/Projects-apps/tiamat-otel
go build ./...
```

No binary is produced (library module). Build verifies compilation.

## Test

```bash
go test ./...
```

Note: As of 2026-03-06, the project has no test files. Tests are a backlog priority.

## Run example

```bash
cd examples/hello
OTEL_SERVICE_NAME=hello OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 go run main.go
```

Requires a local OTLP-compatible collector (e.g., Jaeger with OTLP receiver).

## Updating consumers

When the otel API changes:

1. Make changes in otel
2. In each consumer project (volon, cortex, hadron, nanite), run `go mod tidy`
3. Update call sites to match new signatures
4. Verify with `go build ./...` and `go test ./...` in each consumer

No versioning/tagging is needed for local development (replace directive). For a future registry publish, tag with semver.

## Release process

Not yet established. Currently consumed only via local `replace` directives. A tagging/release process would be needed if the module is published to a Go module proxy.
