# devport-radar

Discover local development services by port, process, and lightweight HTTP fingerprinting.

`devport-radar` is a practical Go CLI for developers and coding agents who need fast visibility into *what is running on localhost right now*.

## Why this exists

Recent dev-tool trends are converging on:
- autonomous agents that spin many local services,
- browser/automation tooling that needs stable targets,
- better local DX around service discovery.

`devport-radar` gives a terminal-first live map of listening services with enough metadata to act quickly.

## Features (v0)

- Scans local listening TCP sockets via `ss -ltnpH`
- Dedupe across IPv4/IPv6 listeners
- Resolves process name + PID when available
- Probes `http://127.0.0.1:<port>` with timeout
- Extracts HTTP status, `Server` header, and page `<title>`
- Infers a coarse service fingerprint (`nextjs`, `vite-dev-server`, `python-web`, etc.)
- Outputs table or JSON
- Optional watch mode with periodic refresh and port appear/disappear delta logs

## Install

### Go install

```bash
go install github.com/agent19710101/devport-radar/cmd/devport-radar@latest
```

### Build locally

```bash
git clone git@github.com:agent19710101/devport-radar.git
cd devport-radar
go build ./cmd/devport-radar
```

## Usage

```bash
# one-shot table
devport-radar

# json output
devport-radar --json

# tune HTTP probe timeout
devport-radar --timeout 1200ms

# live refresh every 3s
devport-radar --watch --interval 3
```

## Example output

```text
PORT   PROCESS          PID    HTTP  FINGERPRINT   TITLE
5432   postgres         22410  -     -             -
6379   redis-server     22301  -     -             -
3000   node             48122  200   nextjs        My App
8080   api              49100  200   go-service    Dev API
```

## Roadmap

- TUI mode with grouped projects and live health badges
- Optional reverse-DNS/project labeling
- Prometheus exporter mode
- macOS support via `lsof` fallback when `ss` is unavailable

## License

MIT
