# Session Validation

`devpulse validate` checks a saved traffic session before it is used in reports, CI jobs, or debugging handoffs.

## Usage

```bash
devpulse validate
devpulse validate --from ./artifacts/session.json
devpulse validate --from ./artifacts/session.json --json
devpulse validate --json > validation-report.json
```

The command exits with status `0` when the session passes validation and `1` when the session is readable but invalid. File loading errors also exit with status 1. Invalid sessions still produce a report when `--json` is selected, making the command suitable for CI artifact collection.

## Checks

Session-level checks run first, then each request is checked in session order. Within a request the fields are checked in the order listed below, so the output is deterministic for the same input.

| Scope | Field | Code | Rule |
| --- | --- | --- | --- |
| session | `version` | `unsupported_version` | Must be `1` |
| session | `createdAt` | `missing_created_at` | Must be a non-zero timestamp |
| request | `time` | `missing_timestamp` | Must be a non-zero timestamp |
| request | `method` | `missing_method` | Must not be empty or whitespace |
| request | `path` | `invalid_path` | Must begin with `/` or equal `*` (after trimming whitespace) |
| request | `status` | `invalid_status` | Must be between 100 and 599 inclusive |
| request | `latency` | `negative_latency` | Must not be negative |
| request | `targetHost` | `missing_target_host` | Must not be empty or whitespace |
| request | `targetPort` | `invalid_target_port` | Must be between 1 and 65535 inclusive |

`bytes` is not validated. The proxy records `-1` when the upstream response has no `Content-Length`.

Validation reads the file with a plain JSON decode, so a session with an unsupported `version` is still reported as an `unsupported_version` issue instead of failing to load.

## JSON output

`--json` prints an envelope containing the `session` path and a `report` object. The report includes `valid`, `request_count`, and an `issues` array, which is empty, not `null`, when the session is valid. Each issue includes a stable `code`, the `field` it concerns, a human-readable `message`, and, for request-level issues, a one-based `request` index.

```json
{
  "session": "./artifacts/session.json",
  "report": {
    "valid": false,
    "request_count": 2,
    "issues": [
      {
        "request": 2,
        "field": "status",
        "code": "invalid_status",
        "message": "HTTP status must be between 100 and 599"
      }
    ]
  }
}
```

Codes are stable identifiers intended for scripts; messages may be reworded.
