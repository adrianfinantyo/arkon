# Arkon Specification

Status: ready-for-agent

## Problem Statement

I run several homelab servers, including older repurposed desktop PCs (such as an Intel Pentium Dual-Core E5400 machine with mechanical storage hard drives). Running these machines 24/7 became an issue in practice: an idle 45W machine running continuously consumes unnecessary electricity, produces constant fan and drive spindle noise, and subjects mechanical drives to constant wear.

Putting these machines to sleep overnight and waking them in the morning sounded simple, but standard Linux tooling proved inadequate across my machines:
1. Conventional tools like `rtcwake` and cron `shutdown` cut power abruptly. Running Docker Compose stacks, background tasks, and databases (PostgreSQL, SQLite, Valkey) faced interrupted queries, torn transactions, and corrupted write-ahead logs.
2. Operating system dirty page buffers did not reliably drain to mechanical disks before power dropped.
3. Custom cron bash scripts were brittle, unversioned, broke whenever container compose names changed, and offered no clean way to pause shutdowns during active maintenance.
4. Heavy management panels (Portainer, Cockpit, Coolify) consume 150MB to 500MB of resident RAM. On a modest machine with only 4GB of RAM, that memory is needed for actual workloads, not an idle web UI that cannot reliably program hardware RTC alarms or survive host power transitions anyway.

I built Arkon to manage host power lifecycles and workload teardowns reliably across my homelabs. I am sharing it as a lightweight, zero-overhead tool for anyone managing similar setups.

## Solution

Arkon is a lightweight, single-binary CLI tool and systemd orchestrator that parameterizes host power transitions, manages pluggable workload teardowns in dependency order, programs hardware RTC wakealarms, and maintains GitOps-driven host systemd units.

Arkon functions as a compiler and execution engine with 0 MB of idle memory overhead:
1. Schedules, teardown profiles, and lifecycle hooks are declared in a single `arkon.yaml` manifest.
2. Running `arkon apply` compiles the manifest into native `/etc/systemd/system/` timer and service units, delegating all scheduling to systemd PID 1 with zero persistent background daemons.
3. When the circadian sleep window arrives, Arkon coordinates a phased container teardown pipeline (stopping application tiers before database infrastructure), flushes dirty filesystem pages via `syscall.Sync()`, programs the Real-Time Clock wakealarm via direct sysfs writes (`/sys/class/rtc/rtc0/wakealarm`), and executes the configured host power action (ACPI S5 poweroff, S3 suspend, or S4 hibernate).
4. Running operations can be protected via timed `arkon inhibit` locks that prevent poweroff during critical jobs and auto-expire without human intervention.
5. Operators and automation inspect host power state, timer countdowns, and active locks through a single borderless left-rail terminal view or a typed JSON envelope (`--json`).

## User Stories

1. As a homelab operator, I want to define daily sleep and wake times in `arkon.yaml`, so that my physical server powers down automatically at night and powers up in the morning.
2. As a homelab operator, I want Arkon to consume 0 MB of resident RAM when idle, so that all system memory remains dedicated to my container workloads.
3. As a homelab operator, I want to run `arkon apply --dry-run`, so that I can inspect generated systemd units and validation errors without needing root access or altering host files.
4. As a system administrator, I want `arkon apply` to validate unit syntax and back up existing units before installation, so that a syntax error cannot break systemd configuration.
5. As a system administrator, I want generated systemd timers to set `Persistent=false`, so that waking a machine after a missed sleep window does not trigger an immediate shutdown storm.
6. As a database administrator, I want containers to stop in ordered profile tiers (such as `apps` then `infra`), so that databases do not receive sudden SIGKILL signals while servicing active connections.
7. As a storage administrator, I want Arkon to invoke a filesystem sync and confirm dirty page counts drain to zero before power drops, so that mechanical disks and journal buffers do not lose uncommitted writes.
8. As a homelab operator, I want Arkon to stop any residual containers not tagged with a profile before power drops, so that orphaned containers are never cut off ungracefully.
9. As a system administrator, I want to program the hardware RTC wakealarm through direct sysfs writes rather than `rtcwake`, so that the system avoids runtime dependencies on `util-linux` and does not spawn unmonitored sub-processes.
10. As a system administrator, I want Arkon to cancel pending RTC alarms by writing `0\n` before setting a new alarm, so that the Linux kernel never returns an `-EBUSY` error.
11. As an operator of a machine configured with local BIOS clock, I want Arkon to parse `/etc/adjtime` during status inspection, so that displayed wake timestamps accurately reflect host wall-clock time.
12. As a safety engineer, I want Arkon to reject any sleep schedule configured within 180 seconds of the wake target, so that the host avoids ACPI S5 shutdown race conditions.
13. As a backup script author, I want to run `arkon inhibit --reason "Nightly backup" --duration 2h`, so that an active backup job prevents the scheduled nightly poweroff from interrupting it.
14. As an operator running daytime tasks, I want Arkon to reject manual `arkon sleep` commands during the configured daytime window unless overridden with `--force`, so that daytime work is not terminated by accident.
15. As an automation engineer, I want expired inhibition locks to clean up automatically without operator action, so that temporary overrides never permanently disable the circadian schedule.
16. As an operator, I want to release an inhibition lock explicitly using `arkon inhibit release <id>`, so that the scheduled sleep can resume immediately after early maintenance completion.
17. As an operator, I want `arkon status` to display host state, timers, RTC wake alarms, and inhibition locks in a vertical left-rail layout, so that information is readable on narrow terminals and mobile SSH clients without line wrapping.
18. As an automation script, I want to pass `--json` to any Arkon command, so that I receive a consistent typed JSON envelope with `success`, `data`, and `error` fields.
19. As an automation script, I want Arkon to exit with code 2 when a power transition is blocked by an active inhibition lock, so that scripts can detect lock states unambiguously.
20. As an unprivileged user, I want `arkon status` to succeed without `sudo`, so that monitoring agents can read system state without elevated credentials.
21. As an AI coding agent, I want to run `arkon schema`, so that I can inspect the canonical JSON Schema for manifests and state files directly without guessing properties.
22. As an operator, I want to define arbitrary pre-teardown, post-teardown, pre-sleep, and post-wake hooks in `arkon.yaml`, so that custom server commands (like disk spindowns or network alerts) run at the right point in the power cycle.
23. As a developer using Podman or plain CLI processes, I want workload teardown decoupled behind a pluggable provider interface, so that Arkon can stop workloads on systems without Docker.
24. As an operator, I want Arkon to search for `arkon.yaml` in standard locations (`--config`, current directory, `/etc/arkon/`), so that invocation is natural across development and production environments.
25. As a system administrator, I want atomic file locking on the state file, so that concurrent CLI commands cannot corrupt runtime state.

## Implementation Decisions

### 1. Zero Resident Architecture and Systemd Integration

Arkon does not run as a background daemon. All recurring scheduling is offloaded to systemd PID 1 by compiling native systemd service and timer unit pairs:
- Service unit: Configured as `Type=oneshot`, invoking `arkon sleep --config <resolved-path>`.
- Timer unit: Configured with `OnCalendar` matching the circadian schedule, with `Persistent=false` to prevent immediate catch-up shutdowns when powering on after a missed sleep window.
- Unit generation: Templates are embedded into the Go binary at compile time.
- Atomic commit: Units are written to a temporary staging directory, validated via template parsing and `systemd-analyze verify`, backed up from `/etc/systemd/system/`, and atomically renamed into place before triggering `systemctl daemon-reload`.

### 2. Manifest Schema (`arkon.yaml`)

The manifest is versioned and structured into modular, open-for-extension sections:

```yaml
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0" # default: /sys/class/rtc/rtc0
  power_action: "poweroff"         # canonical power action: poweroff (ACPI S5), suspend (ACPI S3), hibernate (ACPI S4)

circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]

teardown:
  timeout_seconds: 30
  compose_stacks:
    - path: "/opt/stacks/apps"
  profile_order:
    - "apps"
    - "infra"

hooks:
  pre_teardown: []
  post_teardown:
    - name: "filesystem-sync"
      command: "sync"
      timeout_seconds: 10
      ignore_errors: false
    # Scalar shorthand is also supported: - "sync" expands with default timeout (10s) and ignore_errors (false)
  pre_sleep: []
  post_wake: []

inhibition:
  default_duration: "2h"
  daytime_protection: true
  daytime_start: "07:00" # optional: defaults to circadian.wake_at
  daytime_end: "23:00"   # optional: defaults to circadian.sleep_at
```

Key schema conventions:
- **Hostname resolution:** Hostname is discovered dynamically from the host operating system (`/etc/hostname` or kernel query), rather than declared statically in YAML.
- **Power transition action:** Configured under `host.power_action`, defining whether Arkon shuts down (`poweroff`), suspends to RAM (`suspend`), or hibernates to disk (`hibernate`).
- **Hook syntax:** Lifecycle hooks accept both structured mappings (`name`, `command`, `timeout_seconds`, `ignore_errors`) and scalar string shorthands (e.g., `- "sync"`).
- **Daytime boundaries:** Daytime protection defaults to the circadian `wake_at` and `sleep_at` window, with optional explicit overrides via `daytime_start` and `daytime_end`.

### 3. Pluggable Workload Teardown Provider

The teardown pipeline coordinates workload stopping through a clean interface:

```go
type TeardownProvider interface {
    Name() string
    CheckLiveness(ctx context.Context) error
    StopTier(ctx context.Context, tier string, timeout time.Duration) error
    StopResidual(ctx context.Context, timeout time.Duration) error
}
```

The default provider manages Docker Compose stacks by issuing sequential `docker compose --profile <tier> stop` commands, followed by a residual stop for untagged services. Providers are extensible to Podman Compose, K8s pod drains, and systemd slice targets.

### 4. Filesystem Sync and Buffer Draining

Before executing any power state transition:
- Arkon executes all configured `post_teardown` hooks.
- It calls `syscall.Sync()` to instruct the kernel to flush dirty pages.
- It reads `/proc/meminfo` and waits with a bounded timeout for `Dirty` page bytes to drain to zero or plateau before cutting host power.

### 5. Hardware RTC Wakealarm and ACPI S5 Abstraction

Hardware wake management bypasses external subprocesses:
- Clear existing alarm: Writes `0\n` to `/sys/class/rtc/rtc0/wakealarm`.
- Set relative alarm: Computes delta seconds between current time and target wake time, then writes `+<seconds>\n` to `/sys/class/rtc/rtc0/wakealarm`. This approach avoids BIOS UTC vs local clock translation mismatches during writes.
- Verify retention: Reads back `/sys/class/rtc/rtc0/wakealarm` to verify the hardware counter accepted the target epoch.
- Status inspection: Reads `/etc/adjtime` line 3 (`UTC` or `LOCAL`) to map the sysfs epoch integer to local wall-clock time for display.
- Enforce safety buffer: Rejects sleep executions where the duration between poweroff and wake is less than 180 seconds.

### 6. Inhibition Locks and State Management

- Runtime state is stored at `/var/lib/arkon/state.json` on root storage.
- File access is guarded by an advisory `syscall.Flock` on `/var/lib/arkon/state.lock`.
- State updates write to a temporary file in `/var/lib/arkon/` and swap into place via atomic `os.Rename`.
- Locks record an identifier, creator, human reason, and expiration timestamp.
- Expired locks are purged dynamically on state read.
- Daytime operational windows treat daytime sleep commands as inhibited unless overridden with `--force`.

### 7. Terminal UX, Machine Contract, and Exit Codes

- CLI command surface:
  - `arkon apply`: Compile `arkon.yaml` and synchronize host systemd service and timer units.
  - `arkon status`: Display host power state, RTC alarm countdown, and active inhibition locks.
  - `arkon sleep`: Execute workload teardown pipeline, sync disks, arm hardware RTC, and cut host power.
  - `arkon wake`: Execute post-wake lifecycle hooks upon host resume.
  - `arkon inhibit`: Acquire, list, or release temporary maintenance locks blocking sleep.
  - `arkon schema`: Emit canonical JSON Schema for validation and agent introspection.
  - `arkon version`: Print build version, commit, and date.
- Terminal output: Uses a single, vertical, borderless left-rail layout with colored bullets (`●`) and indented key-value pairs, avoiding table border wrapping on narrow screens.
- JSON envelope: Global `--json` flag formats all output into a typed envelope:
  ```json
  {
    "success": true,
    "data": {},
    "error": null
  }
  ```
- Deterministic exit codes:
  - `0`: Success / OK
  - `1`: General Error
  - `2`: Power Transition Inhibited (Active Lock or Daytime Protection)
  - `3`: Manifest Validation or Compilation Failure
  - `4`: Insufficient Privileges (Root required without `--dry-run`)

## Testing Decisions

### Test Philosophy
Tests must exercise external behavior through stable seams, verifying observable outcomes (exit codes, output envelopes, written unit files, and state records) rather than testing private struct fields or internal execution details.

### Testing Seams
1. **Primary Command Seam (`HostEnvironment`):**
   The primary entrypoint is tested by passing a mockable environment adapter:
   ```go
   type HostEnvironment interface {
       FS() fs.FS
       WriteFile(path string, data []byte, perm os.FileMode) error
       ReadFile(path string) ([]byte, error)
       Exec(ctx context.Context, name string, args ...string) ([]byte, error)
       SyncFS() error
       Now() time.Time
   }
   ```
   This allows end-to-end testing of `apply`, `status`, `sleep`, and `inhibit` without needing root permissions, real systemd daemons, or hardware RTC sysfs devices.

2. **Internal Workload Seam (`TeardownProvider`):**
   The teardown pipeline is tested using a test double provider that records stop order, verifies timeout compliance, and simulates container stop failures.

3. **Pure Logic Modules (Zero Seams Needed):**
   Manifest YAML parsing, circadian schedule delta calculations, and state lock expiration math are pure functions tested directly without mocks or adapters.

### Modules Tested
- **Manifest:** Syntax parsing, field validation, default value population, schema verification.
- **Compiler:** Unit template rendering, atomic staging, rollback behavior, privilege rejection.
- **Teardown Coordinator:** Profile sequencing, residual service stopping, hook execution order, error handling.
- **RTC Subsystem:** Sysfs path validation, relative syntax formatting, zero-clearing write, adjtime parsing.
- **State Manager:** Lock acquisition, lock release, lock auto-expiration, concurrent flock contention, atomic replacement.
- **CLI Dispatcher:** Argument parsing, help and version output, left-rail formatting, `--json` serialization, exit code contracts.

## Out of Scope

- Continuous background HTTP daemon or persistent web dashboard (monitoring is handled by external tools like Beszel).
- Multi-node clustering, distributed consensus, or Kubernetes federation (Arkon is designed for single-node homelab systems).
- Direct block-level filesystem formatting or partition management.
- Packaging distribution (Debian `.deb` packaging and GoReleaser pipeline), which remains in the project roadmap backlog.

## Further Notes

- Target Hardware: Intel Pentium Dual-Core E5400 (2 cores, 2 threads @ 2.7GHz), 3.8 GiB RAM, 256GB Root SSD, mechanical HDDs.
- BIOS Configuration: Hardware power-on requires ErP/EuP power-saving modes to be disabled in the motherboard BIOS so the ATX `+5VSB` rail maintains power to the ACPI RTC wake logic while in S5 soft-off.
- Hardware RTC Comparator Limit: Some legacy motherboards only support 24-hour hardware RTC wake comparators lacking the ACPI `DAY_ALRM` field. Schedules longer than 24 hours must be clamped or flagged accordingly.
