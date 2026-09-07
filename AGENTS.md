# go-otel

An opinionated OpenTelemetry bootstrap for Go services. It wires OTLP HTTP
trace export plus opt-in metrics and logs, installs W3C trace-context and
Baggage propagators, and ships the `hollis.*` span/metric taxonomy, GenAI
semantic-convention helpers, a `Recorder` over the instrument set, HTTP and
MCP propagation, slog trace correlation and signal-aware shutdown. It is
bootstrap and taxonomy, not a telemetry backend.

## Start Here

- `README.md` covers the quickstart and every opt-in flag.
- `hotel.go` owns `Init` and shutdown; the Go package name is `hotel`.
- `tracing.go`, `metrics.go` and `logging.go` own the three signal pipelines.
- `recorder.go` is the opinionated layer over the `hollis.*` instruments.
- `genai/attributes.go` holds the GenAI semantic conventions.
- `redaction/redaction.go` owns the sensitive-attribute denylist.
- `propagation/propagation.go` owns HTTP and MCP context propagation.
- `examples/hello/main.go` is a runnable service bootstrap.

## Commands

```bash
gofmt -l .
go vet ./...
go test -race -count=1 ./...
golangci-lint run
```

There is no CI workflow in this repo. `.golangci.yml` is errors-only on
purpose and supports `golangci-lint run --new` for changed lines.

## Boundaries

Prompt and completion content is sensitive. `redaction.Denylist` names
`gen_ai.content.prompt` and `gen_ai.content.completion`, and redaction is
driven by `HOLLIS_OTEL_REDACT_PROMPTS`. Adding a GenAI attribute that can
carry user content without adding it to the denylist exports that content to
whatever backend is configured — `TestDenylist` is the guard.

Every signal beyond traces is opt-in and must stay genuinely off when not
enabled: `TestInitWithoutMetricsDoesNotExportMetrics`,
`TestInitWithoutLogsDoesNotExportLogs` and
`TestInitWithRuntimeMetricsIsNoOpWithoutMetricsEnabled` all exist to catch a
default that quietly starts exporting.

The wire taxonomy is `hollis.*` while the Go identifier is `hotel`. That split
is deliberate — do not rename one to match the other.

Metric label shapes are pinned by tests
(`TestRecorderMessageLabelShapeDivergence`,
`TestRecorderQueueDepthSignedDelta`). Cardinality and delta signedness are
contracts with the metrics backend, not local choices.
