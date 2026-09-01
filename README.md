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
- Cross-platform Go implementation

## Installation
Requires **Go 1.24+**.

```bash
git clone https://github.com/yatinannam/devpulse.git
cd devpulse
go build -o devpulse ./cmd/devpulse
```

On Windows:
```powershell
go build -o devpulse.exe ./cmd/devpulse
```

## Quick start
```bash
devpulse ports
devpulse traffic
devpulse status
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
| `devpulse watch` | Continuously refresh service/traffic health |
| `devpulse config` | View or change persistent defaults |
| `devpulse version` | Print the current build version |

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
DevPulse is in active development toward **v0.1.0**. The first release focuses on reliable local discovery, HTTP traffic capture, correlation, diagnostics, and a lightweight terminal workflow.

## License
DevPulse is licensed under the MIT License.