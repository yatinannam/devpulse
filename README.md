# DevPulse

**Local development observability for developers who want to know what is running, what is receiving traffic, and what needs attention.**

DevPulse combines two small tools into one workflow:
- **PortDoctor** — discovers local listening services, processes, and common frameworks.
- **APIWatch** — records HTTP traffic and surfaces errors, slow requests, and duplicate calls.

Together:
```text
ports/processes → services → HTTP traffic → endpoints → diagnostics
```

## Features
- Local TCP port and process discovery
- Framework/service identification
- HTTP reverse-proxy traffic capture
- Persistent traffic sessions
- Service ↔ endpoint correlation
- Error, slow-request, and duplicate-request detection
- Live `watch` mode
- Persistent local configuration
- Cross-platform Go implementation (Linux, Windows, and macOS)

## Installation

### Windows

Download the latest Windows x64 archive from the GitHub Releases page, extract `devpulse.exe`, and add its directory to `PATH`.

Verify:

```powershell
devpulse --version
```

### macOS / Linux

Download the matching archive from GitHub Releases, extract `devpulse`, and place it on your `PATH`.

Verify:

```bash
devpulse --version
```

### Build from source

Requires **Go 1.22+**.

```bash
git clone https://github.com/yatinannam/devpulse.git
cd devpulse
go build -o devpulse ./cmd/devpulse
```
## Quick start
```bash
cd my-project
devpulse init
devpulse traffic
devpulse status
devpulse recent
devpulse doctor
devpulse watch
```

`traffic` defaults to `http://localhost:3000` as the upstream and listens on `:9090`. Point your application/client at the proxy, generate requests, then stop with **Ctrl+C**. The session is saved locally.

## Commands
| Command | Purpose |
| --- | --- |
| `devpulse ports` | List local listening ports and processes |
| `devpulse ports --watch` | Watch for port changes |
| `devpulse traffic` | Capture HTTP traffic through the proxy |
| `devpulse status` | Correlate services with captured traffic |
| `devpulse doctor` | Analyze a captured session |
| `devpulse recent` | Show the most recent captured requests |\n| `devpulse watch` | Continuously refresh service/traffic health |
| `devpulse config` | View or change persistent defaults |
| `devpulse version` | Print the current build version |

## Getting started

`devpulse init` inspects the current project, detects common project types, finds a matching local HTTP service when available, and stores the detected target as the DevPulse default.

Use `devpulse help`, `devpulse --help`, or `devpulse -h` to display the command reference. Use `devpulse --version` or `devpulse -v` to print the version.

## Configuration
Configuration is stored at `~/.devpulse/config.json`.
```bash
devpulse config --target http://localhost:8080
devpulse config --listen :9090
devpulse config --watch-interval 5s
```

Environment variables:
- `DEVPULSE_CONFIG` — override the configuration file path.
- `DEVPULSE_SESSION` — override the traffic session path.

## Architecture
```text
┌──────────────┐
│ PortDoctor   │──→ ports/processes/frameworks
└──────────────┘
         │
         ▼
┌──────────────────────┐
│ Service correlation  │
└──────────────────────┘
         ▲
         │
┌──────────────┐
│ APIWatch     │──→ requests/endpoints/latency/errors
└──────────────┘
         │
         ▼
┌──────────────┐
│ Doctor       │──→ actionable findings
└──────────────┘
```

## Development
```bash
go test ./...
go vet ./...
```
GitHub Actions runs both checks on pushes and pull requests to `main`.

## Project status

DevPulse is in active development toward **v0.2**.

## License
DevPulse is licensed under the MIT License.