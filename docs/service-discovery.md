# Service Discovery Reference

PortDoctor has two stages: enumerate listening TCP sockets and probe each listener to determine whether it speaks HTTP. Discovery is local-only and does not scan arbitrary network hosts.

## Listener enumeration

| Platform | Source | Notes |
| --- | --- | --- |
| Linux | `ss -ltnp` | Reads listening TCP sockets and process metadata. |
| macOS | `lsof -nP -iTCP -sTCP:LISTEN -F pcn` | Field-oriented output; no matches is an empty result. |
| Windows | `netstat -ano` and `tasklist /FO CSV /NH` | Correlates listening sockets with process names using PID. |

IPv4 and IPv6 rows for the same port and PID are merged into one listener. Two different processes that share a port remain separate. Unsupported operating systems return an explicit unsupported-platform error rather than an empty result.

The parsers are pure Go functions with fixture tests, so output parsing is exercised on every CI platform.

## HTTP probe sequence

Each listener is probed concurrently:

1. Dial `127.0.0.1:<port>`. If it does not accept a connection, try `[::1]:<port>`. The address that answers is used for the HTTP requests and displayed service URL.
2. Send `HEAD /` without following redirects.
3. If response headers do not identify the service, send `GET /` with an HTML-oriented Accept header. The response body is capped at 8 KiB for fingerprinting.

The probe uses its own HTTP transport, disables keep-alive, and does not rely on the user's proxy environment variables. Each probe carries `X-DevPulse-Probe: 1`. A running DevPulse proxy answers that marker directly with 204 and an `X-DevPulse-Proxy` header, rather than forwarding it to the application or recording it as user traffic.

## Probe budgets

The command determines which timeout profile is used:

| Caller | Dial timeout | Shared HTTP budget |
| --- | ---: | ---: |
| `init` | 250 ms per address | 2 s for HEAD and GET |
| `status`, `watch`, `baseline`, `diff`, `anomalies` | 300 ms per address | 600 ms for HEAD and GET |

The longer `init` budget accommodates development servers that compile their first page on request. Repeated inspection commands use shorter budgets so a listener that accepts TCP but never responds cannot delay every refresh.

All listeners are probed concurrently; the budgets apply per listener rather than serially across the whole machine.

## Framework fingerprints

Signals are checked in order; the first match wins.

| Signal | Classification |
| --- | --- |
| `X-DevPulse-Proxy` response header | DevPulse proxy |
| `Server: uvicorn` | Python / FastAPI |
| `Server: gunicorn` | Python / Gunicorn |
| `Server` contains `next`, or page includes `__NEXT_DATA__` / `_next/static` | Node / Next.js |
| `Server` contains `vite`, or page includes `@vite` / `vite/client` | Node / Vite |
| Page body mentions `fastapi` | Python / FastAPI |
| `Server` or `X-Powered-By` contains `express` | Node / Express |
| Jetty/Tomcat server marker, `X-Application-Context`, or Spring Whitelabel page | Java / Spring |
| `Server` contains `caddy` or `nginx` | Caddy / Nginx |
| Any other HTTP response | Generic HTTP service |

The ordering avoids classifying a frontend as FastAPI merely because its page mentions the backend it calls.

## How init selects a target

`init` detects the project type from common marker files, searching upward from the current directory. It stops at the repository root or filesystem root. It then chooses from discovered HTTP services using these rules, in order:

1. Prefer the conventional port for the detected project type: Next.js 3000, Vite 5173, Python 8000, Go or Java 8080.
2. If there is only one HTTP service, choose it.
3. Otherwise prefer the first listening conventional port from 3000, 5173, 8000, 8080, 8081, and 9000.
4. Prefer a service with an identified framework over a generic HTTP service, then the lowest port.

Review the target printed by `init` before starting `traffic`; automatic selection is a convenience, not proof that the chosen service is the intended upstream.

## Limits to keep in mind

- Probes use plain HTTP, not TLS. An HTTPS-only listener can be listed as non-HTTP. Configure an explicit `https://` target for traffic capture.
- Only loopback addresses are probed. A service bound only to a LAN address is not fingerprinted.
- A service may accept TCP but fail to answer within the selected HTTP budget. It can appear as non-HTTP during a quick refresh and be identified later.
- Discovery reports the local process/listener view; it does not establish that a service is reachable from another machine.

For symptoms and suggested next steps, see [Troubleshooting](troubleshooting.md).
