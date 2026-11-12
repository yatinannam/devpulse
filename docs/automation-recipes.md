# Automation Recipes

DevPulse commands can be used in local scripts and CI jobs without a running dashboard. Capture traffic first, then validate the saved session before feeding it to analysis or export commands.

## Recommended pipeline

```bash
SESSION=./artifacts/session.json

# Stop early when the input is unreadable or fails session validation.
devpulse validate --from "$SESSION"

# Store machine-readable analysis as separate CI artifacts.
devpulse summary --from "$SESSION" --json > ./artifacts/summary.json
devpulse doctor --from "$SESSION" --json > ./artifacts/doctor.json
devpulse report --from "$SESSION" --format json --out ./artifacts/report.json

# Export only failed API requests when that is all the job needs.
devpulse export --from "$SESSION" --status 500 --path-prefix /api/ --out ./artifacts/server-errors.csv
```

The report and export commands read the saved session; they do not start a proxy, discover services, or contact the application.

## Make findings fail a CI step

`validate` exits 1 when a readable session is invalid. In contrast, `doctor` and `anomalies` intentionally exit 0 when they produce findings, so a CI job that treats findings as a failure must inspect their JSON.

The following example uses `jq` to make the policy explicit:

```bash
set -eu

SESSION=./artifacts/session.json

devpulse validate --from "$SESSION" --json > ./artifacts/validation.json
jq -e '.report.valid == true' ./artifacts/validation.json > /dev/null

devpulse doctor --from "$SESSION" --json > ./artifacts/doctor.json
jq -e '.findings | length == 0' ./artifacts/doctor.json > /dev/null
```

For baseline regression checks, run `devpulse anomalies --baseline <baseline> --session "$SESSION" --json` and inspect the `findings` array in the output. The command reports results; choose your own threshold or policy for which findings should fail a build.

## Use stdout in pipelines

`export` writes CSV to stdout by default:

```bash
devpulse export --from "$SESSION" --status 500 | gzip > ./artifacts/errors.csv.gz
```

Use `--out -` with `report` to write Markdown or JSON to stdout:

```bash
devpulse report --from "$SESSION" --out - --title "CI traffic" > ./artifacts/report.md
devpulse report --from "$SESSION" --format json --out - > ./artifacts/report.json
```

Keep standard output for data and standard error for operational messages. When redirecting data, do not merge stderr into the output file with `2>&1`.

## Exit status

| Code | Meaning |
| ---: | --- |
| `0` | Command succeeded. `doctor` and `anomalies` can still have findings. |
| `1` | Runtime failure, unreadable input, or `validate` found an invalid session. |
| `2` | Usage error, such as an unknown flag or invalid option. |

Use JSON fields as the automation contract rather than parsing human-readable terminal messages. Validation issue codes and report field names are intended for scripts; human-readable messages can be revised.

## Artifacts and data handling

Traffic sessions and derived outputs can contain raw request paths and query strings. Review them before uploading CI artifacts or attaching them to a ticket. DevPulse does not redact these values automatically and does not upload them itself.

The recorder keeps the newest 10,000 requests by default. Check the capture's final message for dropped requests: a structurally valid session can still represent only the newest portion of a longer run.

Related references:
- [Session Validation](session-validation.md)
- [Diagnostic Profiles](diagnostic-profiles.md)
- [Markdown Traffic Reports](markdown-reports.md)
- [Traffic CSV Export](traffic-csv-export.md)
