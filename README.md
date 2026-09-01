# DevPulse

Lightweight local development observability for understanding what your application is doing right now.

DevPulse connects two views of local development:

- Ports and processes — which local services are running and which process owns each port.
- HTTP traffic — what requests are flowing through those services.

The long-term goal is correlation:

frontend :5173 -> API :3000 -> database :5432

## Status

### v0.1 — Port inspection

Implemented:

- devpulse ports
- Windows TCP listener discovery via netstat
- Process lookup via tasklist
- Linux TCP listener discovery via ss

Planned:

- devpulse ports --watch
- devpulse traffic
- devpulse doctor
- devpulse watch

## Development

Requires Go 1.24+.

Build:

    go build ./cmd/devpulse

Run:

    devpulse ports

## Principles

- Local-first
- No account required
- No cloud dependency
- Small, composable commands
- Deterministic diagnostics before AI
