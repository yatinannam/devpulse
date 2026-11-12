# Capture Lifecycle and Recovery

This page describes what happens between starting `devpulse traffic` and getting a durable session file.

## Start a capture

The proxy defaults to `127.0.0.1:9090` and forwards to `http://localhost:3000`. Select an explicit upstream when needed:

```bash
devpulse traffic --target http://localhost:8080
```

Use the address printed in the startup banner as the browser or client base URL. A listen port of `0` asks the operating system for a free port; the banner shows the actual port after the listener is bound.

The default session is `~/.devpulse/session.json`. Set `DEVPULSE_SESSION` to select another default location, or pass `--from` to commands that consume sessions.

## Request retention

The recorder keeps the newest 10,000 requests by default. Set `--max-requests` to a larger or smaller positive value:

```bash
devpulse traffic --max-requests 50000
```

When the limit is exceeded, the oldest requests are dropped to make room for new ones. DevPulse warns on stderr the first time a request is dropped and includes the total number discarded in the final save message. A capture can therefore be valid while still being incomplete; check the dropped count before using it for load or frequency analysis.

Values below 1 are usage errors (exit status 2). A value of 1 is allowed and retains only the newest request.

## When a request is recorded

Each proxied request is recorded once when its response body finishes streaming, or when the proxy fails the request. The `bytes` field is the response body data actually streamed, not simply the upstream `Content-Length`. It may be lower than that header when a transfer is interrupted.

The session stores:
- The original request method and request target, including the query string.
- The upstream status, including DevPulse's proxy-generated 499, 502, and 508 cases.
- Latency from proxy receipt until the response body finishes.
- Bytes streamed to the client and the parsed upstream host and port.

See [Traffic Session Format](session-format.md) for the field definitions and compatibility rules.

## Graceful shutdown

On Ctrl+C or SIGTERM, DevPulse stops accepting new connections, closes connections that were accepted but never sent a request, and waits up to five seconds for active requests. Requests that finish during this window are included in the snapshot. Streams still active at the deadline, such as a long-running server-sent event, are closed; their final byte count may be partial.

The warning about requests being cut off is emitted only when a request is still active at the deadline. Pressing Ctrl+C a second time exits immediately without saving, so use the first interrupt for normal shutdown.

## How the session file is saved

DevPulse serializes the capture to a temporary file in the destination directory, applies owner-only permissions where supported, flushes the file to disk, closes it, and renames it over the session path. This keeps readers from seeing a partially written session.

On Windows, replacing a file that another process has open can fail temporarily. DevPulse retries sharing and access errors for about one second. If the rename still fails, the complete synced temporary file is kept beside the intended session and the error includes its exact path.

Do not remove that temporary file until recovery is complete. Close readers such as another DevPulse command or an editor, move the reported temporary file to the intended session location, and validate it:

```bash
devpulse validate --from ~/.devpulse/session.json
```

Use the path printed by the save error when moving the file; the command above assumes recovery has already placed it at the default location.

## Before sharing or analyzing a capture

1. Check the final save message for the number of dropped requests.
2. Run `devpulse validate --from <session>`.
3. Inspect paths and query strings for tokens, identifiers, or other sensitive values.
4. Generate a report or export only the data needed for the investigation.

Validation checks structure and request invariants. It does not redact or modify captured data.
