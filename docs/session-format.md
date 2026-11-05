# Traffic Session Format

DevPulse stores captures as versioned JSON. The current schema uses version `1`, a creation timestamp, and an array of request records.

```json
{
  "version": 1,
  "createdAt": "2026-01-02T03:04:05Z",
  "requests": [
    {
      "time": "2026-01-02T03:04:06Z",
      "method": "GET",
      "path": "/health",
      "status": 200,
      "latency": 12000000,
      "bytes": 24,
      "targetHost": "localhost",
      "targetPort": 8080
    }
  ]
}
```

Duration values such as `latency` are encoded as integer nanoseconds by Go's JSON encoder for `time.Duration` fields. Timestamps use RFC 3339 / RFC 3339 Nano encoding through `time.Time`.

## Request fields

Each proxied request is recorded exactly once, when its response body has finished streaming to the client or when the proxy fails it.

| Field | Meaning |
| --- | --- |
| `time` | When the proxy received the request. |
| `method` | The client's request method. |
| `path` | The request URI exactly as the client sent it to the proxy, including the query string. A path prefix in `--target` is not included: with `--target http://localhost:3000/api`, a client request to `/users?x=1` is forwarded to `/api/users?x=1` but recorded as `/users?x=1`. |
| `status` | The upstream status code. `502` when the upstream could not be reached or the proxy failed the request. `499` (client closed request) when the client cancelled before the upstream answered; these are not logged as proxy errors. A client that disconnects while the body is streaming keeps the upstream status, with `bytes` counting what was streamed. |
| `latency` | Nanoseconds from when the proxy received the request until the response body finished streaming (it reached EOF or was closed). For a `101 Switching Protocols` upgrade it stops at the upgrade response; for a proxy error it stops when the error occurred. |
| `bytes` | Response body bytes actually streamed, not the upstream `Content-Length`. It is `0` for responses without a body (`HEAD`, `204`, `304`), for upgrades and for proxy errors, and can be smaller than `Content-Length` when the transfer was cut short. |
| `targetHost`, `targetPort` | The upstream host and port parsed from `--target`. The port defaults to `80` for `http` and `443` for `https`. |

Sessions captured by earlier DevPulse releases measured `latency` only until the upstream response headers, stored the upstream `Content-Length` in `bytes` (`-1` when it was unknown, for example for chunked responses), and stored the upstream-rewritten URI in `path`.

## Compatibility

- `version: 1` is the only supported version at present.
- Readers reject unknown versions rather than guessing at their meaning.
- Use `devpulse validate` to check required fields and request invariants before consuming a session in automation.

## Data handling

Session files can contain request paths and query strings. Treat them as local diagnostic artifacts and review them before sharing. Validation does not redact or modify captured values.
