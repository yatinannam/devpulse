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

## Compatibility

- `version: 1` is the only supported version at present.
- Readers reject unknown versions rather than guessing at their meaning.
- Use `devpulse validate` to check required fields and request invariants before consuming a session in automation.

## Data handling

Session files can contain request paths and query strings. Treat them as local diagnostic artifacts and review them before sharing. Validation does not redact or modify captured values.
