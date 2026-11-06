# Markdown Traffic Reports

The `devpulse report` command turns a saved traffic session into a standalone Markdown or JSON artifact suitable for pull requests, CI artifacts, incident notes, dashboards, or debugging handoffs.

## Usage

```bash
devpulse report
devpulse report --from ./artifacts/session.json --out ./artifacts/traffic.md
devpulse report --title "Release smoke test" --top 10 --out ./artifacts/smoke-report.md
devpulse report --from ./artifacts/session.json --out -
devpulse report --top 0 --out ./artifacts/summary.md
devpulse report --format json --out ./artifacts/traffic.json
devpulse report --format json --out -
```

## JSON output

Use `--format json` to emit machine-readable report data. JSON includes a stable set of snake_case field names for report metadata, aggregate summary metrics, endpoint statistics, status-code counts, slow requests, and diagnostic findings. Collection fields are always JSON arrays: an empty collection, such as `slowest_requests` with `--top 0`, is written as `[]`, never `null`. When `--out` is omitted, JSON output defaults to `devpulse-report.json`; an explicit output path always takes precedence.

Durations are integer nanoseconds and timestamps are RFC3339 strings. Strings are written literally: characters such as `&`, `<` and `>` are not HTML-escaped.

| Field | Contents |
| --- | --- |
| `title`, `session_path` | Report heading and the session file it was built from |
| `session_created_at`, `generated_at` | When the session was created and when the report was built |
| `summary` | `total`, `errors`, `error_rate` (0 to 1), `average`, `p50`, `p95`, `p99` |
| `status_codes[]` | `status`, `count` |
| `endpoints[]` | `method`, `path`, `requests`, `errors`, `error_rate`, `average_latency` |
| `slowest_requests[]` | `time`, `method`, `path`, `status`, `latency`, `bytes`, `target_host`, `target_port` |
| `findings[]` | `level`, `title`, `detail`, `advice` |

`endpoints[].path` is the normalized endpoint path described below. `slowest_requests[].path` keeps the raw captured path, including any query string.

## Markdown report sections

Sections appear in this order:

- **Summary:** request and error counts, error rate, average latency, and P50/P95/P99 latency.
- **Top endpoints:** request count, error count/rate, and average latency for each method and normalized path. Normalization matches `status`, `baseline` and `diff`: the query string and fragment are removed and numeric and UUID segments become `:id`, so `/items/1?tab=a` and `/items/2` share the row `/items/:id`.
- **HTTP status codes:** observed status distribution in numeric order.
- **Slowest requests:** timestamp, method, path, status, and latency for up to `--top` requests. With `--top 0` the table is replaced by a line saying the details were omitted with `--top 0`; an empty session shows a `No requests` row instead.
- **Diagnostic findings:** deterministic findings from the default DevPulse doctor profile and actionable advice.

Endpoints are sorted by request volume, then error count, then path and method. Status codes are sorted numerically. Table cells escape pipes and flatten newlines so captured paths cannot corrupt Markdown tables. The title and session path are flattened to a single line and their Markdown characters (`` \ ` * _ [ ] < > # | ``) are backslash-escaped, so a `--title` such as `"Smoke\n## Fake"` stays part of the heading instead of adding a section.

## Output and data handling

Markdown reports default to `devpulse-report.md`; JSON reports default to `devpulse-report.json`. An explicit `--out` always wins: `--format json --out devpulse-report.md` writes JSON to `devpulse-report.md`. Set `--out -` or `--out ""` to write either format to standard output for pipelines. File output is written through a temporary file and renamed into place; generated reports use owner-only permissions where supported.

Reports can include raw request paths and diagnostic details. Review them before sharing externally. The command does not upload data or modify the source session: it refuses an `--out` path that names the source session file, including case variants on Windows and macOS, hard links, and symlinks.

## Exit behavior

The command exits 2 for a usage error (an unknown flag, a positional argument, an unsupported `--format`, or a negative `--top`), which is reported before the session is loaded. It exits 1 if the session cannot be read or the report cannot be rendered or saved. A value of `--top 0` disables slow-request entries while retaining the section header and an omission note.
