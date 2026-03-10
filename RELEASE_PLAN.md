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

## Next milestones

### v0.5.0 — terminal operations mode

Tracking issue: [#2](https://github.com/agent19710101/devport-radar/issues/2)

Scope:
- Minimal TUI dashboard (service list + health/status)
- Basic sort/filter interactions (port/process/http status)
- Watch mode integration in TUI

Quality gate:
- Snapshot/golden coverage for core TUI rendering paths
- README includes usage + keybindings
- `go test ./...` green

### v0.6.0 — integrations and automation ✅

Tracking issue: [#3](https://github.com/agent19710101/devport-radar/issues/3)

Delivered:
- Prometheus exporter mode (`--metrics-addr`, `--metrics-path`)
- Optional service labels/aliases from local file (`--aliases-file`)
- Docs/examples for alias config and metrics scraping
- Test coverage for alias parsing/application and metrics rendering

Quality gate: `go test ./...` + `go vet ./...` green.

### v0.7.0 — next reliability and UX polish

Tracking issue: TBD

Scope:
- Add machine-friendly `/healthz` endpoint in metrics mode
- Optional sort controls for table/TUI (`--sort port|process|http`)
- Docs: minimal Grafana panel + alert examples for `devport_radar_service_up`

Quality gate:
- Deterministic tests for sort ordering + endpoint output
- No regressions in existing JSON/NDJSON contracts
