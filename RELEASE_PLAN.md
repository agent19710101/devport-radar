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

### v0.6.0 — integrations and automation

Tracking issue: [#3](https://github.com/agent19710101/devport-radar/issues/3)

Scope:
- Prometheus exporter mode
- Optional service labels/aliases from local config
- Example alerting/dashboard snippets in docs

Quality gate:
- Integration tests for exporter output
- Backward-compatibility notes for config fields
- `go test ./...` + `go vet ./...` green

### v0.7.0 — agent-runtime presets and richer fingerprints

Tracking issue: [#12](https://github.com/agent19710101/devport-radar/issues/12)

Scope:
- Optional profile presets for common agent/dev stacks
- Additional non-invasive fingerprint hints (framework/runtime cues)
- Keep core scan/probe behavior backward-compatible

Quality gate:
- New hints covered with deterministic tests
- No regressions in existing JSON/NDJSON contracts
