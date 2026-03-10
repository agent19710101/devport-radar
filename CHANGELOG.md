# Changelog

## v0.7.0 - 2026-03-10

### Added
- Fallback to `lsof` when `ss` fails at runtime (not only when missing).
- `/healthz` endpoint in metrics server mode.
- Optional metrics auth guard via `--metrics-token` (Bearer auth for `/metrics` and `/healthz`).
- Sort controls for table/watch rendering: `--sort port|process|http`.

### Improved
- Unicode/display-width safe title truncation in terminal output.
- Grafana panel and alert starter docs for `devport_radar_service_up`.
- Roadmap/release-plan markdown cleanup (removed active milestone `TBD`, synced status).
