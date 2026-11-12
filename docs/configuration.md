# Configuration Reference

DevPulse uses a small JSON configuration for the proxy target, listen address, and watch refresh interval. You can keep settings per project or use a user-wide configuration.

## Defaults

| Key | Default | Meaning |
| --- | --- | --- |
| `listen` | `127.0.0.1:9090` | Local proxy address. Loopback-only by default. |
| `target` | `http://localhost:3000` | Application URL to which the proxy forwards requests. |
| `watch_interval` | `2s` | Refresh interval used by `devpulse watch`. |

A config file is optional. When no file exists, DevPulse uses these defaults.

## Which file is used

DevPulse selects one configuration path in this order:

1. `DEVPULSE_CONFIG`, when set. This is a hard override, even when the specified file does not exist yet.
2. The nearest `.devpulse/config.json`, searching upward from the current working directory to the filesystem root.
3. `~/.devpulse/config.json`.

When neither an explicit override nor a project config exists, the user-global path is selected. `devpulse config` prints and updates the active configuration. `devpulse init` writes a project config in the detected project root.

Set an explicit path before running DevPulse:

**macOS / Linux**

```bash
export DEVPULSE_CONFIG="$HOME/.config/devpulse/work.json"
devpulse config
```

**PowerShell**

```powershell
$env:DEVPULSE_CONFIG = "$HOME/.config/devpulse/work.json"
devpulse config
```

The override is useful for separate environments, scripted runs, or keeping a project-specific configuration outside a checked-out repository.

## Update settings

```bash
devpulse config
devpulse config --target https://localhost:8443
devpulse config --listen 127.0.0.1:9090
devpulse config --watch-interval 5s
```

Each supplied value is checked against the rules used by `traffic` and `watch`. Invalid values are rejected with usage status 2 and are not saved.

- A target must be an `http://` or `https://` URL with a host. If it specifies a port, that port must be in the range 1–65535.
- A listen address must use `host:port` with a numeric port. Use `127.0.0.1` for local-only access; an empty host binds all interfaces.
- A watch interval must be a positive Go duration, such as `500ms`, `2s`, or `5s`.

## Project configuration

Run `devpulse init` from inside a project to detect its root and save `.devpulse/config.json` there. The root is found by searching upward for project markers, stopping at the repository root or filesystem root. This lets a command started from a nested directory use the same project settings.

When no project marker is found, `init` reports an unknown project and writes the project config in the current directory. Review the selected target before starting a capture.

## Compatibility and warnings

- Config files are UTF-8 JSON. A leading UTF-8 BOM is ignored, including the BOM written by Windows PowerShell 5.1 when a file is re-saved.
- Known keys match without regard to case. For example, `"TARGET"` is treated as `target`.
- Unknown keys are ignored and reported as sorted warnings. When DevPulse rewrites a config file, it preserves unknown keys so an older binary does not erase values written by a newer version.
- An invalid `watch_interval` in an existing file is reported and replaced with the default interval at runtime. Correct the file to remove the warning.

Configuration files are written with owner-only permissions where the operating system supports them. The config does not contain credentials by default; avoid adding secrets to a file that may be copied into a project or support bundle.
