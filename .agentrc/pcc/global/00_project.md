---
intent: pcc_global
project: otel
updated_at: "2026-03-06"
---

# Project Identity

## What it is

otel is a shared Go module (`github.com/hollis-labs/otel`) that provides standardized OpenTelemetry instrumentation for all Fragments Engine applications. It is a thin wrapper around the OTel Go SDK that enforces the Observability Contract.

## Goals

- **Single source of truth** for OTel bootstrap across Volon, Cortex, Hadron, Nanite, and Carrier (Python uses its own SDK but follows the same contract).
- **One-call initialization** via `tiamatotel.Init()` — sets up trace provider, propagator, and exporter from env vars and option overrides.
- **Domain span helpers** — pre-built spans for agent steps, tool calls, memory reads/writes that enforce the contract's naming and attribute requirements.
- **GenAI semantic conventions** — helpers for LLM model calls following the OTel GenAI semconv spec.
- **Attribute redaction** — denylist-based span processor that strips prompts, completions, and secrets by default.
- **Cross-service propagation** — W3C traceparent for HTTP, custom `_traceparent` key for MCP tool calls.

## Non-goals

- Not an application — has no `main()` (only an `examples/hello/` demo).
- Does not implement business logic or domain models.
- Does not manage its own collector or backend — assumes an OTLP-compatible endpoint exists.
- Does not handle Python instrumentation (Carrier uses `opentelemetry-python` directly).

## Active configuration

- **Module**: `github.com/hollis-labs/otel`
- **Go version**: 1.25.0
- **OTel SDK version**: v1.41.0
- **Volon project ID**: `otel`
- **Volon version**: 0.2.0
- **Repo path**: `~/Projects-apps/tiamat-otel`
- **Created**: 2026-03-05 as part of the OTel + GenAI Emitters initiative
