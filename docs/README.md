# DevPulse Documentation

Start with the [root README](../README.md) for the product overview, installation, command reference, architecture, and development instructions.

## Operator guides

| Guide | Use it when |
| --- | --- |
| [Troubleshooting](troubleshooting.md) | The proxy will not start, requests loop, a service appears non-HTTP, or a capture cannot be saved. |
| [Configuration](configuration.md) | You need to choose project vs global settings, set `DEVPULSE_CONFIG`, or understand validation and compatibility behavior. |
| [Capture lifecycle](capture-lifecycle.md) | You need to understand request retention, shutdown, session durability, or Windows recovery. |
| [Service discovery](service-discovery.md) | You need details about listener enumeration, HTTP probing, fingerprints, or target selection. |
| [Automation recipes](automation-recipes.md) | You want to validate sessions, create CI artifacts, or make diagnostic findings affect a build. |

## Data and analysis references

| Reference | Contents |
| --- | --- |
| [Traffic Session Format](session-format.md) | Versioned JSON schema, request fields, encoding, and data-handling notes. |
| [Session Validation](session-validation.md) | Validation rules, stable issue codes, and JSON output envelope. |
| [Diagnostic Profiles](diagnostic-profiles.md) | Thresholds, duplicate detection, profile selection, and JSON findings. |
| [Markdown Traffic Reports](markdown-reports.md) | Markdown and JSON report generation, output behavior, and data handling. |
| [Traffic CSV Export](traffic-csv-export.md) | CSV schema, filters, timestamps, and safe output behavior. |

## Suggested reading paths

**First capture:** [Quick start](../README.md#quick-start) → [Capture lifecycle](capture-lifecycle.md) → [Traffic Session Format](session-format.md).

**Investigate a problem:** [Troubleshooting](troubleshooting.md) → [Diagnostic Profiles](diagnostic-profiles.md) → [Session Validation](session-validation.md).

**Use DevPulse in CI:** [Automation recipes](automation-recipes.md) → [Session Validation](session-validation.md) → [Traffic CSV Export](traffic-csv-export.md) or [Markdown Traffic Reports](markdown-reports.md).

**Tune project setup:** [Configuration](configuration.md) → [Service discovery](service-discovery.md).

The command behavior described here follows the current `main` branch. Check the root README and release notes when upgrading between versions.
