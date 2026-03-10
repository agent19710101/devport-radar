# devport-radar

Discover local development services by port, process, and lightweight HTTP fingerprints.

## Problem

Modern local development stacks (apps, databases, queues, agent runtimes, browser tooling) spin up many listeners quickly. It is often unclear what is running, where, and which service is healthy.

`devport-radar` provides a terminal-first map of listening TCP services with process metadata and fast HTTP probing.

## Status

- Current release: **v0.4.6**
- Platform: Linux (`ss` backend)
- Maturity: early, actively iterating in small releases
- Merge readiness requires CI matrix (`go1.24.x`, `go1.25.x`) with `gofmt`, `go vet`, `go test`, and `go test -race` (latest Go)

## Features

- Scan local listening TCP sockets via `ss -ltnpH`
- Dedupe duplicate listeners (e.g., IPv4/IPv6 overlap)
- Resolve process name + PID when available
- Probe multiple local targets per port (bind host + loopbacks) with configurable timeout
- Capture HTTP status, `Server`, page `<title>`, and coarse fingerprint
- Output in table or JSON
- Watch mode with appear/disappear delta logs
- Configurable watch change detection (`--watch-detect port|port-process`)
- Structured watch JSON mode (`--watch --json`) emitting NDJSON `appeared`/`disappeared`/`snapshot`/`error` events
- Focus scans via `--ports` list/range filter
- Limit output to responsive HTTP services via `--only-http`
- Optional probe bypass via `--no-http-probe` for faster port/process inventory

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

# tighten probe timeout
devport-radar --timeout 600ms

# show only endpoints with HTTP responses
devport-radar --only-http

# skip HTTP probing for faster socket/process-only scans
devport-radar --no-http-probe

# live mode every 3 seconds
devport-radar --watch --interval 3

# reduce title truncation in narrow terminals
devport-radar --title-width 24
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

- Ensure `ss` is available and working: `ss -ltnpH`
- Run with sufficient privileges if process/PID fields are empty (some distros restrict process metadata)
- If you only need listener inventory, bypass probes: `devport-radar --no-http-probe`

### HTTP probe false negatives

- Increase timeout for slower local services: `devport-radar --timeout 2s`
- Use `--only-http` only when you explicitly want probe-positive services
- For transient watch-time probe/scan errors, default mode retries automatically; use `--watch-strict` only for fail-fast automation

### Watch mode automation

- NDJSON includes `error` events; consumers should handle and continue unless strict mode is enabled
- Use `--watch-detect port-process` if process restarts on the same port should be treated as changes

## Roadmap

- [ ] TUI mode with grouped projects and health badges
- [ ] Project labels/aliases for stable service naming
- [ ] Prometheus exporter mode
- [ ] macOS fallback backend (`lsof`) when `ss` is unavailable
- [ ] See scoped milestones in [RELEASE_PLAN.md](./RELEASE_PLAN.md)

## License

MIT
