# Changelog

All notable changes to `go-otel` are recorded in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

While the project is pre-1.0 (v0.x), minor releases may contain backward-incompatible
changes; breaking changes are always called out under "Changed" or "Removed".

## [v0.1.0] — 2026-05-10

First public release.

### Added
- `WithServiceNamespace(namespace string) Option` for setting the
  `service.namespace` resource attribute. Also reads from the
  `OTEL_SERVICE_NAMESPACE` environment variable. When empty (the default),
  no `service.namespace` attribute is emitted.
- Public `README.md`, `CHANGELOG.md`, `doc.go`, and `.gitignore`.

### Changed
- `service.namespace` is no longer hardcoded. Previous releases set it to
  `"fragments-engine"` unconditionally; this is now configurable via
  `WithServiceNamespace` or `OTEL_SERVICE_NAMESPACE` and defaults to unset.
  Migration: callers that relied on the previous default should add
  `feotel.WithServiceNamespace("fragments-engine")` (or the appropriate
  namespace for their deployment) to their `Init` call.
- Package-level godoc on `feotel` rewritten to describe the public API.
- `internal.NewResource` now takes a `serviceNamespace` parameter and
  emits the attribute only when non-empty. The `internal/` package is
  not part of the public API.

### Removed
- Internal repository configuration files that should not ship in a
  public release: `.agentrc/`, `.claude/`, `CLAUDE.md`, `agentrc.yaml`,
  `AUDIT_RESULTS.md`, `lefthook.yml`, and the internal-only
  `docs/observability-contract.md`.

### Security
- Bumped `golang.org/x/net` v0.50.0 → v0.54.0 to pick up the fixes for
  GO-2026-4918 (HTTP/2 transport infinite loop on bad
  `SETTINGS_MAX_FRAME_SIZE`) and GO-2026-4559 (HTTP/2 server panic on
  certain frames). Indirect bumps of `golang.org/x/sys` and
  `golang.org/x/text` followed via `go mod tidy`.

### Fixed
- `go.mod` `go` directive bumped from 1.25.0 to 1.26.1 to match the
  current toolchain. Build / vet / `go test -race` / `govulncheck` of
  in-lib symbols are clean on Go 1.26.1; remaining govulncheck findings
  are all standard-library issues fixed in Go 1.26.2 / 1.26.3
  (`crypto/tls`, `crypto/x509`, `net`, `net/http`, `html/template`) and
  are not actionable at the library level — consumers should compile
  against Go 1.26.3 or later.
