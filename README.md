# devport-radar

Discover local development services by port, process, and lightweight HTTP fingerprints.

## Problem

Modern local development stacks (apps, databases, queues, agent runtimes, browser tooling) spin up many listeners quickly. It is often unclear what is running, where, and which service is healthy.

`devport-radar` provides a terminal-first map of listening TCP services with process metadata and fast HTTP probing.

## Status

- Current release: **v0.6.0**
- Platform: Linux (`ss`) + macOS fallback (`lsof`)
- Maturity: early, actively iterating in small releases
- Merge readiness requires CI matrix (`ubuntu-latest` + `macos-latest`, Go `1.24.x`/`1.25.x`) with `gofmt`, `go vet`, `go test`, and `go test -race` (latest Ubuntu Go lane)

## Features

- Scan local listening TCP sockets via `ss -ltnpH` with automatic `lsof` fallback when `ss` is unavailable
- Dedupe duplicate listeners (e.g., IPv4/IPv6 overlap)
- Resolve process name + PID when available
- Probe multiple local targets per port (bind host + loopbacks) with configurable timeout
- Capture HTTP status, `Server`, page `<title>`, and coarse fingerprint
- Output in table or JSON
- Watch mode with appear/disappear delta logs
- Configurable watch change detection (`--watch-detect port|port-process`)
- Structured watch JSON mode (`--watch --json`) emitting NDJSON `appeared`/`disappeared`/`snapshot`/`error` events
- Minimal terminal dashboard mode (`--watch --tui`) for live refresh in-place
- Focus scans via `--ports` list/range filter or `--profile` presets (`agent|web|data`)
- Limit output to responsive HTTP services via `--only-http`
- Optional probe bypass via `--no-http-probe` for faster port/process inventory
- Richer service fingerprints for common local runtimes (Ollama, Open WebUI, Qdrant, Redis, Postgres, MySQL, MongoDB, Grafana, Prometheus)
- Optional service aliases via local file (`--aliases-file`)
- Prometheus scrape endpoint mode (`--metrics-addr`, `--metrics-path`)

## Install

### Go install

```bash
go install github.com/agent19710101/devport-radar/cmd/devport-radar@latest
```

### Build from source

```bash
git clone git@github.com:agent19710101/devport-radar.git
cd devport-radar
go build ./cmd/devport-radar
```

## Examples

```bash
# one-shot table output
devport-radar

# output as json (single snapshot array)
devport-radar --json

# watch as NDJSON events (appeared/disappeared + snapshot)
devport-radar --watch --json --interval 3

# treat process swaps on same port as service changes
devport-radar --watch --watch-detect port-process --interval 2

# probe only common app ports
devport-radar --ports 3000,5173,8080,5432

# probe a full range
devport-radar --ports 8000-8100

# focus on common local agent runtime stack ports
devport-radar --profile agent

# tighten probe timeout
devport-radar --timeout 600ms

# show only endpoints with HTTP responses
devport-radar --only-http

# skip HTTP probing for faster socket/process-only scans
devport-radar --no-http-probe

# live mode every 3 seconds
devport-radar --watch --interval 3

# minimal terminal dashboard mode (auto-refresh in place)
devport-radar --watch --tui --interval 3

# reduce title truncation in narrow terminals
devport-radar --title-width 24

# apply stable aliases for ports from a local file
cat > aliases.txt <<'EOF'
3000=frontend
5432=postgres-db
8080=api
EOF
devport-radar --aliases-file ./aliases.txt

# expose Prometheus metrics for scrape
devport-radar --metrics-addr :9317 --aliases-file ./aliases.txt
curl -s localhost:9317/metrics
```

## Automation Contract (v0.x, stable unless noted)

### Exit behavior

- `0`: successful scan/watch run
- `1`: invalid arguments, scan/probe failures, or JSON encoding failures

### `--json` one-shot schema

`devport-radar --json` emits a single JSON array of services:

```json
[
  {
    "port": 8080,
    "protocol": "tcp",
    "process": "my-app",
    "pid": 12345,
    "http_status": 200,
    "server": "Caddy",
    "title": "Dashboard",
    "fingerprint": "http-service",
    "alias": "frontend",
    "scanned_at": "2026-03-10T06:30:00Z"
  }
]
```

### `--watch --json` NDJSON event schema

Each line is a JSON object with:

- `type`: `appeared` | `disappeared` | `snapshot` | `error`
- `timestamp`: RFC3339 time
- `port`: present for delta events
- `service`: present for delta events
- `services`: present for `snapshot` events
- `error`: present for `error` events

### `--watch-detect`

- `port` (default): identity is only the port; process restarts on same port are not emitted as changes.
- `port-process`: identity is `port+pid` (fallback `port+process`), so process swaps on the same port are emitted.

### `--watch-strict`

- Default watch behavior is resilient: transient scan failures are emitted/logged and the next tick retries.
- Set `--watch-strict` to fail-fast and exit on the first scan error.

### `--tui`

- Valid with `--watch`; renders a minimal terminal dashboard with clear-screen redraws.
- Intended for interactive use (not machine-readable pipelines).

### `--aliases-file`

- Optional plain-text file mapping stable labels to ports.
- Format: one mapping per line, `<port>=<alias>` (comments allowed with `#`).
- Aliases are applied to table output, JSON snapshots, watch events, and metrics labels.

### `--metrics-addr` / `--metrics-path`

- Starts an HTTP server that exposes Prometheus metrics (`text/plain; version=0.0.4`).
- Endpoint performs a fresh scan on each scrape and emits:
  - `devport_radar_services_total`
  - `devport_radar_service_up{port,process,fingerprint,alias}`
  - `devport_radar_service_http_status{...}` (only when probe succeeded)

### `--no-http-probe`

- Disables all HTTP GET probing and fingerprint enrichment.
- Output still includes listener/process metadata (`port`, `process`, `pid`, `bind`, `scanned_at`).

### Script example (jq)

```bash
# print newly appeared ports in watch mode
(devport-radar --watch --json --interval 2 \
  | jq -r 'select(.type=="appeared") | .service.port')
```

## Troubleshooting

### `scan failed` or empty output

- Preferred backend uses `ss`: verify with `ss -ltnpH`
- On hosts without `ss` (e.g. macOS), `devport-radar` falls back to `lsof -nP -iTCP -sTCP:LISTEN`
- Run with sufficient privileges if process/PID fields are empty (some distros restrict process metadata)
- If you only need listener inventory, bypass probes: `devport-radar --no-http-probe`

### HTTP probe false negatives

- Increase timeout for slower local services: `devport-radar --timeout 2s`
- Use `--only-http` only when you explicitly want probe-positive services
- For transient watch-time probe/scan errors, default mode retries automatically; use `--watch-strict` only for fail-fast automation
- Redirect responses are recorded as-is; probes do not follow off-host redirects

### Watch mode automation

- NDJSON includes `error` events; consumers should handle and continue unless strict mode is enabled
- Use `--watch-detect port-process` if process restarts on the same port should be treated as changes

### Flag combination validation

- `--tui`, `--watch-strict`, and non-default `--watch-detect` require `--watch`
- `--tui` cannot be combined with `--json`
- `--only-http` cannot be combined with `--no-http-probe`
- `--interval` must be greater than zero

## Roadmap

- [x] Minimal TUI mode (`--watch --tui`)
- [x] Project labels/aliases for stable service naming
- [x] Prometheus exporter mode
- [x] macOS fallback backend (`lsof`) when `ss` is unavailable
- [ ] See scoped milestones in [RELEASE_PLAN.md](./RELEASE_PLAN.md)

## License

MIT
