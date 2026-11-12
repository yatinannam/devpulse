# Troubleshooting

This guide covers the failure modes that are easiest to confuse with application bugs when using APIWatch and PortDoctor.

## The proxy does not start

Check the target and listen address separately:

```bash
devpulse traffic --target http://localhost:3000 --listen 127.0.0.1:9090
```

- `--target` must be an absolute `http://` or `https://` URL with a host. If a port is present, it must be between 1 and 65535. A bare value such as `localhost:3000` is rejected because it has no scheme.
- `--listen` must be `host:port`. Use a numeric port; `0` asks the OS for a free port. An empty host, as in `:9090`, binds all interfaces. The default is the safer loopback-only `127.0.0.1:9090`.
- A target that resolves to the proxy's own listen address is rejected as a usage error. Do not point the proxy back at itself.

The banner prints the actual bound proxy address. When using port `0`, use that printed port in your browser or client.

## Requests return 508 Loop Detected

DevPulse adds `X-DevPulse: 1` to requests it forwards. A request that arrives with that header has already passed through a DevPulse proxy, so the proxy answers with HTTP 508 rather than forwarding it again.

This catches loops that cannot be identified from the literal `--target` and `--listen` values, such as a target that refers to the same machine through a LAN address. It also means that chaining one DevPulse proxy behind another is not supported. Remove the extra proxy hop or configure the target to point directly at the application.

## A healthy service appears as non-HTTP

PortDoctor probes loopback listeners with plain HTTP; it does not perform TLS handshakes. An HTTPS-only service can therefore appear as a non-HTTP listener even though it is healthy. When capturing traffic to such a service, configure an explicit HTTPS target:

```bash
devpulse traffic --target https://localhost:8443
```

A listener bound only to a LAN address is also outside the loopback probe scope. PortDoctor tries `127.0.0.1`, then `::1`; it does not probe arbitrary network interfaces.

## A service is missing from a quick status refresh

Discovery uses different budgets for one-time setup and repeated inspection:

| Caller | TCP dial timeout | Shared HTTP probe budget |
| --- | ---: | ---: |
| `init` | 250 ms per loopback dial | 2 s for HEAD and GET |
| `status`, `watch`, `baseline`, `diff`, `anomalies` | 300 ms per loopback dial | 600 ms for HEAD and GET |

The longer `init` budget allows a development server that compiles its first page on request to identify itself. Repeated commands use a shorter budget so a listener that accepts TCP but never answers cannot stall every refresh. If a service is still starting, wait for it to answer and run the command again.

## Interpret proxy status codes

| Status | DevPulse meaning |
| ---: | --- |
| `499` | The client cancelled before the upstream answered. This is recorded but is not treated as a proxy failure. |
| `502` | The upstream could not be reached, the proxy failed the request, or the upstream sent an invalid status below 100. |
| `508` | The incoming request already carried the DevPulse forwarded-request marker and was stopped to prevent a proxy loop. |

A 4xx or 5xx response is still a captured upstream response when the upstream returned it. Use `devpulse doctor` to inspect diagnostic findings, and distinguish application responses from proxy-generated failures.

## Capture ends but the session was not replaced

On Windows, another process can temporarily prevent the session file from being replaced while it is open. DevPulse retries the rename for about one second. If the replacement still fails, the complete, synced capture is kept in a sibling file named like `.session-*.tmp`, and the error reports its path.

Close programs that may be holding the session open, then recover the kept capture by moving the reported temporary file to the intended session path. Do not delete the temporary file until the recovered session has been checked:

```bash
devpulse validate --from ./path/reported/by/the/error.tmp
```

Use the exact temporary path printed by DevPulse; the example above is only a placeholder.

## A saved session is rejected by a downstream command

Run validation before generating reports or exports:

```bash
devpulse validate --from ./artifacts/session.json
devpulse validate --from ./artifacts/session.json --json
```

A session reader ignores a leading UTF-8 BOM, including one written by Windows PowerShell 5.1. It does not silently accept an unsupported session version or repair invalid request records. See [Session Validation](session-validation.md) and [Traffic Session Format](session-format.md).

## More references

- [Capture lifecycle](capture-lifecycle.md) explains request limits, shutdown, and recovery.
- [Service discovery](service-discovery.md) explains the listener and HTTP probe stages.
- [Configuration](configuration.md) explains target, listen, and config-file selection.
