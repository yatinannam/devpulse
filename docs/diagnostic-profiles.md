# Diagnostic Profile Reference

This page is a concise operational reference for `devpulse doctor`. The root README explains the main workflow; its [architecture](../README.md#architecture) and [known limitations](../README.md#known-limitations) sections describe implementation details and limitations.

## Invocation

```bash
devpulse doctor
devpulse doctor --profile strict
devpulse doctor --profile permissive --from ./testdata/session.json
devpulse doctor --profile strict --json
```

The default session path is resolved using `DEVPULSE_SESSION` when set, otherwise `~/.devpulse/session.json`. Use `--from` to point at an explicit captured session. Profile names are case-insensitive.

## Threshold matrix

| Signal | Default | Strict | Permissive |
| --- | --- | --- | --- |
| Error status | >= 400 | >= 400 | >= 500 |
| Slow request | >= 500 ms | >= 250 ms | >= 1 s |
| Duplicate minimum | 3 | 2 | 4 |
| Duplicate window | 3 s | 5 s | 2 s |

A duplicate finding groups entries by exact HTTP method and raw request path. The requests are sorted by timestamp and a window of the profile's duration slides across them. A finding is reported when any window contains at least the duplicate minimum, so a sustained request loop, or a burst followed by a late straggler, is still detected. The finding reports the count and time span of the densest burst. Query strings are not stripped by the doctor duplicate detector; endpoint normalization used by status and baseline aggregation is a separate operation.

## JSON envelope

With `--json`, standard output is a JSON object:

```json
{
  "session": "/path/to/session.json",
  "profile": "strict",
  "requests": 12,
  "findings": [
    {
      "level": "WARN",
      "title": "Slow requests",
      "detail": "2 request(s) took 250ms or more",
      "advice": "Check backend logs and downstream calls for the slow endpoint before optimizing the client."
    }
  ]
}
```

The example is illustrative, not output from a live capture. If there are no findings, `findings` is an empty array. The profile does not rewrite or filter the session; it only changes which diagnostic rules fire.

## Choosing a profile

- Use **default** for routine local development.
- Use **strict** when investigating latency or duplicate-fetch behavior during development.
- Use **permissive** when expected client errors or short polling intervals would otherwise make the output noisy.

Treat findings as investigation prompts. They do not prove a defect, and a clean report does not establish that an application meets its performance objectives.
