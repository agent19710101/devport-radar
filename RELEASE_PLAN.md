# RELEASE_PLAN

Minimal v0.x release path for `devport-radar`.

## Completed baseline

### v0.4.x — usability/reliability baseline ✅ (shipped through v0.4.7)

Delivered:
- macOS fallback backend (`lsof`) when `ss` is unavailable
- Stable JSON/NDJSON contract tests
- CI hardening (`gofmt`, `go vet`, `go test`, race lane)
- Watch resiliency (`error` events + optional strict mode)
- Probe and scan UX improvements (`--only-http`, `--no-http-probe`)

### v0.5.0 — terminal operations mode ✅

Tracking issue: [#2](https://github.com/agent19710101/devport-radar/issues/2)

Delivered:
- Minimal TUI dashboard (service list + health/status)
- Watch mode integration in TUI

Quality gate: `go test ./...` green.

### v0.6.0 — integrations and automation ✅

Tracking issue: [#3](https://github.com/agent19710101/devport-radar/issues/3)

Delivered:
- Prometheus exporter mode (`--metrics-addr`, `--metrics-path`)
- Optional service labels/aliases from local file (`--aliases-file`)
- Docs/examples for alias config and metrics scraping
- Test coverage for alias parsing/application and metrics rendering

Quality gate: `go test ./...` + `go vet ./...` green.

### v0.7.0 — reliability and UX polish ✅

Tracking issues:
- [#20](https://github.com/agent19710101/devport-radar/issues/20) `/healthz` endpoint
- [#21](https://github.com/agent19710101/devport-radar/issues/21) sort controls
- [#22](https://github.com/agent19710101/devport-radar/issues/22) Grafana/alert docs
- [#23](https://github.com/agent19710101/devport-radar/issues/23) markdown status hygiene

Delivered:
- Machine-friendly `/healthz` endpoint in metrics mode
- Optional sort controls for table/watch views (`--sort port|process|http`)
- Optional metrics auth guard (`--metrics-token`)
- Unicode-safe title truncation in terminal output
- Grafana panel + alert starter docs
- Markdown plan/roadmap cleanup (no `TBD` placeholders for current milestone)

Quality gate:
- Deterministic tests for sort ordering + endpoint auth/health output
- No regressions in existing JSON/NDJSON contracts

## Next milestones

### v0.8.0 — operational depth

Scope:
- richer process metadata views and filters
- configurable metric label allowlist for high-cardinality control
- optional watch event file sink

Quality gate:
- backward-compatible JSON contract
- `go test ./...` + `go vet ./...` green
