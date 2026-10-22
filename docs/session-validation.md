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

- Session schema version is `1`NaN`session` path and a `report` object. The report includes `valid`, `request_count`, and an `issues` array. Each issue includes a stable `code`, `field`, `message`, and optionally a one-based `request` index.

Issues follow session order and a fixed field-check order, so output remains deterministic for the same input.
