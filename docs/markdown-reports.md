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

Use `--format json` to emit machine-readable report data. JSON includes a stable set of snake_case field names for report metadata, aggregate summary metrics, endpoint statistics, status-code counts, slow requests, and diagnostic findings. When `--out` is omitted, JSON output defaults to `devpulse-report.json`; an explicit output path always takes precedence.

## Markdown report sections

- **Summary:** request and error counts, error rate, average latency, and P50/P95/P99 latency.
- **Top endpoints:** request count, error count/rate, and average latency for each method/path pair.
- **HTTP status codes:** observed status distribution in numeric order.
- **Slowest requests:** timestamp, method, path, status, and latency for up to `--top` requests.
- **Diagnostic findings:** deterministic findings from the default DevPulse doctor profile and actionable advice.

Endpoints are sorted by request volume, then error count, then path and method. Status codes are sorted numerically. Table cells escape pipes and flatten newlines so captured paths cannot corrupt Markdown tables.

## Output and data handling

Markdown reports default to `devpulse-report.md`; JSON reports default to `devpulse-report.json`. Set `--out -` to write either format to standard output for pipelines. File output is written through a temporary file and renamed into place; generated reports use owner-only permissions where supported.

Reports can include raw request paths and diagnostic details. Review them before sharing externally. The command does not upload data or modify the source session.

## Exit behavior

The command exits non-zero if the session cannot be read, the report cannot be rendered or saved, or `--top` is negative. A value of `--top 0` disables slow-request entries while retaining the section header.
