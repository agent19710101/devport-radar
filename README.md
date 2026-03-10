# devport-radar

Discover local development services by port, process, and lightweight HTTP fingerprints.

## Problem

Modern local development stacks (apps, databases, queues, agent runtimes, browser tooling) spin up many listeners quickly. It is often unclear what is running, where, and which service is healthy.

`devport-radar` provides a terminal-first map of listening TCP services with process metadata and fast HTTP probing.

## Status

- Current release: **v0.3.0**
- Platform: Linux (`ss` backend)
- Maturity: early, actively iterating in small releases

## Features

- Scan local listening TCP sockets via `ss -ltnpH`
- Dedupe duplicate listeners (e.g., IPv4/IPv6 overlap)
- Resolve process name + PID when available
- Probe `http://127.0.0.1:<port>` with configurable timeout
- Capture HTTP status, `Server`, page `<title>`, and coarse fingerprint
- Output in table or JSON
- Watch mode with appear/disappear delta logs
- Configurable watch change detection (`--watch-detect port|port-process`)
- Structured watch JSON mode (`--watch --json`) emitting NDJSON `appeared`/`disappeared`/`snapshot` events
- Focus scans via `--ports` list/range filter

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

# live mode every 3 seconds
devport-radar --watch --interval 3

# reduce title truncation in narrow terminals
devport-radar --title-width 24
```

## Roadmap

- [ ] TUI mode with grouped projects and health badges
- [ ] Project labels/aliases for stable service naming
- [ ] Prometheus exporter mode
- [ ] macOS fallback backend (`lsof`) when `ss` is unavailable
- [ ] See scoped milestones in [RELEASE_PLAN.md](./RELEASE_PLAN.md)

## License

MIT
