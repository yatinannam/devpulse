# Traffic CSV Export

Use `devpulse export` to turn a saved DevPulse traffic session into a CSV file for spreadsheets, notebooks, dashboards, or CI artifacts. The command reads the saved session and does not contact application services.

## Examples

```bash
# Print every request as CSV to stdout
devpulse export

# Save a copy to disk
devpulse export --from ./artifacts/session.json --out ./artifacts/traffic.csv

# Keep only failed POST requests under an API path
devpulse export --method POST --status 500 --path-prefix /api/ --out ./artifacts/errors.csv

# Export a bounded time window (timestamps must be RFC3339)
devpulse export --since 2026-10-28T10:00:00Z --until 2026-10-28T11:00:00Z

# Limit the number of matching rows
devpulse export --method GET --limit 100 --out ./artifacts/recent-gets.csv
```

## CSV schema

The first row is a stable header with these columns:

| Column | Meaning |
| --- | --- |
| `timestamp` | Request timestamp normalized to UTC in RFC3339Nano format |
| `method` | HTTP method |
| `path` | Captured request path |
| `status` | HTTP response status code |
| `latency_ms` | Request latency expressed in milliseconds to three decimal places |
| `bytes` | Response bytes recorded by DevPulse |
| `target_host` | Target host |
| `target_port` | Target port |

CSV values are escaped using Go's standard CSV writer, so commas, quotes, and newlines in paths or values remain valid CSV. A session with no matching requests still exports its header row.

## Filters

- `--method` compares HTTP methods without regard to case or surrounding whitespace.
- `--status` matches one status code from 100 through 599.
- `--path-prefix` selects paths beginning with the supplied prefix.
- `--since` includes requests at or after the RFC3339 timestamp.
- `--until` includes requests at or before the RFC3339 timestamp.
- `--limit` caps the number of matching requests written; zero means unlimited.

Filters can be combined. The limit is applied after the other filters, preserving the order of requests in the source session. A reversed time range and invalid status or limit values are rejected.

## Output and data handling

By default, output goes to stdout, making it easy to pipe into other tools. Use `--out path.csv` to write a file. File output is written to a temporary file in the destination directory and renamed into place, and the resulting file is restricted to the current user where supported. The command refuses to overwrite the source session if the input and output paths resolve to the same location.

CSV may contain raw paths and host metadata. Review it before sharing externally. DevPulse does not upload the exported data.
