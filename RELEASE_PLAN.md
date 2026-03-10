# RELEASE_PLAN

Minimal v0.x release path for `devport-radar`.

## v0.4.0 — UX and portability baseline

Scope:
- macOS fallback backend (`lsof`) when `ss` is unavailable
- Stable JSON schema notes in README
- Improve scan/probe error messaging for common local failure modes

Quality gate:
- `go test ./...` green
- Linux + macOS basic smoke run documented

## v0.5.0 — Terminal operations mode

Scope:
- TUI dashboard (service list + health/status indicators)
- Basic sort/filter interactions (port/process/http status)
- Watch mode integration in TUI

Quality gate:
- Snapshot/golden coverage for core TUI rendering paths
- Manual UX walkthrough documented in README

## v0.6.0 — Integrations and automation

Scope:
- Prometheus exporter mode
- Optional service labels/aliases from local config
- Example alerting/dashboard snippets in docs

Quality gate:
- Integration tests for exporter output
- Backward-compatibility notes for config fields
