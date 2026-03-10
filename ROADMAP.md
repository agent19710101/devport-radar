# ROADMAP

## Shipped (through v0.7.0)

- [x] Port-focused scan filter (`--ports` list/range)
- [x] Structured watch delta output mode (`--watch --json` emits NDJSON events)
- [x] Golden tests for table rendering behavior
- [x] Configurable watch change detection mode (port-only vs process+port)
- [x] TUI dashboard mode (`--watch --tui`)
- [x] macOS/backend fallback (`lsof`) when `ss` is unavailable or fails
- [x] Prometheus exporter endpoint (`--metrics-addr`, `--metrics-path`)
- [x] Metrics auth guard (`--metrics-token`) and `/healthz`
- [x] Service labels/aliases from local config (`--aliases-file`)
- [x] Sort controls (`--sort port|process|http`)
- [x] Unicode-safe title truncation in terminal output
- [x] Docs starter for Grafana panel + alert rule

## Next (v0.8.0)

- [ ] Optional output sinks for watch events (file/webhook)
- [ ] Richer service metadata groups (runtime tags)
- [ ] Config file support for default flags/profiles
