# CLI & Exit Code Reference

Arkon provides a deterministic CLI command surface designed for both human terminal operators and automated agents.

---

## Global Flags

The following flags apply across all subcommands:
- `-c, --config <path>`: Explicitly specify path to `arkon.yaml`. Bypasses automatic discovery hierarchy.
- `--json`: Format all output as a structured JSON envelope to stdout. Diagnostics and progress messages are routed to stderr.
- `--no-color`: Disable ANSI color escape sequences in text output.
- `-v, --verbose`: Enable detailed debug logging to stderr.
- `-h, --help`: Display command usage and available flags.

---

## Subcommands

### `arkon apply`
Compiles the declarative manifest into native systemd units, validates unit syntax, and promotes the manifest to active host authority.

- **Privilege:** Root (`sudo arkon apply`) required for unit installation and canonical symlink creation.
- **Flags:**
  - `--dry-run`: Compiles units in memory, validates syntax, and prints generated units without writing to disk or modifying systemd. Runs cleanly without root.
- **Exit Codes:**
  - `0`: Units compiled, validated, and installed successfully (or dry-run passed).
  - `3`: Manifest validation failure or invalid unit syntax.
  - `4`: Insufficient privileges (EUID is not 0 and not dry-run).

### `arkon status`
Collects and displays current host hardware telemetry, circadian scheduling, RTC wakealarm state, container pipelines, and active inhibition locks.

- **Privilege:** Non-root safe. Runs without `sudo`.
- **Manifest Discovery:** Resolves the active host manifest from `/var/lib/arkon/state.json` and `/etc/arkon/arkon.yaml`. Detects and surfaces local directory drift notices if a local `arkon.yaml` diverges from the active host authority.
- **Exit Codes:**
  - `0`: Status collected and rendered successfully.
  - `1`: Unexpected host query error.
  - `3`: Explicitly specified configuration file (`-c <path>`) is invalid or missing.

### `arkon sleep`
Coordinates the full host sleep sequence:
1. Validates daytime operational protection window (aborts unless `--force` is provided).
2. Checks for active inhibition locks (aborts unless `--force` is provided).
3. Invokes the workload teardown coordinator to stop containers in sequential profile order.
4. Stops residual containers across all stacks via catch-all profile wildcards (`--profile "*"`).
5. Invokes `syscall.Sync()` and polls `/proc/meminfo` until dirty filesystem cache flushes to disk.
6. Programs the next scheduled circadian wake epoch into `/sys/class/rtc/rtc0/wakealarm`.
7. Triggers the configured ACPI power transition (`poweroff`, `suspend`, or `hibernate`).

- **Privilege:** Root (`sudo arkon sleep`) required for container teardown and power transition.
- **Flags:**
  - `--dry-run`: Simulates the entire sleep progression without stopping containers, modifying sysfs, or cutting power.
  - `--force`: Bypasses daytime protection windows and active inhibition locks.
- **Exit Codes:**
  - `0`: Sleep transition completed (or dry-run succeeded).
  - `1`: Workload teardown failure, filesystem sync failure, or poweroff trigger error.
  - `2`: Sleep inhibited by active maintenance lock or daytime operational window.
  - `3`: Manifest validation failure or broken active manifest.

### `arkon up`
Coordinates morning boot orchestration when the host starts:
1. Executes configured `post_wake` hooks (such as drive health checks or detaching degrading disks).
2. Starts declared container stacks in configured profile order (`startup.profiles`).

- **Privilege:** Root (`sudo arkon up`) required.
- **Flags:**
  - `--dry-run`: Simulates execution without invoking hooks or starting containers.
- **Exit Codes:**
  - `0`: Startup completed successfully.
  - `1`: Hook execution or container startup failure.
  - `3`: Manifest validation failure.

### `arkon wake`
Executes configured `post_wake` lifecycle hooks independently. Useful when resuming from ACPI S3 (suspend) or ACPI S4 (hibernate) without running a full cold-boot container startup.

- **Privilege:** Root (`sudo arkon wake`) required.
- **Flags:**
  - `--dry-run`: Simulates hook execution without spawning processes.
- **Exit Codes:**
  - `0`: Hooks executed successfully.
  - `1`: Hook failure where `ignore_errors` is false.
  - `3`: Manifest validation failure.

### `arkon inhibit`
Manages temporary maintenance locks that prevent circadian sleep.

- **Privilege:** Non-root safe (writes to `/var/lib/arkon/state.json`).
- **Flags:**
  - `--reason <text>`: Human-readable explanation for the lock (required when creating).
  - `--duration <duration>`: Duration string (such as `30m`, `2h`, `4h`). Defaults to `inhibition.default_duration` in manifest.
  - `--list`: List all currently active locks and their remaining time.
- **Exit Codes:**
  - `0`: Lock acquired or listed successfully.
  - `1`: Invalid flag parameters or state persistence error.

### `arkon schema`
Outputs the Draft 2020-12 JSON Schema for `arkon.yaml` to stdout.

- **Privilege:** Non-root safe.
- **Exit Codes:**
  - `0`: Schema printed successfully.

---

## Deterministic Exit Codes

Scripts and automation can rely on fixed numeric exit codes:

| Code | Label | Meaning |
| :--- | :--- | :--- |
| `0` | Success | Command completed successfully, or dry-run verified cleanly. |
| `1` | General Failure | Runtime execution error (process failure, command error, I/O error). |
| `2` | Inhibited / Protected | Sleep prevented by active maintenance lock or daytime window. |
| `3` | Validation Failure | Invalid manifest syntax, missing file, or schema error. |
| `4` | Insufficient Privileges | Command requires root privileges (EUID 0). |

---

## Machine Contract (`--json`)

Passing `--json` guarantees that stdout contains only a single JSON envelope:

```json
{
  "command": "status",
  "success": true,
  "exit_code": 0,
  "timestamp": "2026-10-19T23:00:00Z",
  "data": { ... }
}
```

In the event of an error, stdout contains:

```json
{
  "command": "sleep",
  "success": false,
  "exit_code": 2,
  "error": "sleep transition inhibited: active lock 'Restic offsite backup' expires in 1h 45m",
  "timestamp": "2026-10-19T23:00:00Z"
}
```
