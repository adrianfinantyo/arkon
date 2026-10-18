# [01] Declarative Manifest Schema & Systemd Compiler

Type: grilling
Status: resolved
Blocked by: none

## Question

How should the `arkon.yaml` manifest be structured to represent all host lifecycle parameters (schedules, timezone, daytime locks, teardown profiles, and pluggable hooks), and how should the compiler validate and write native `/etc/systemd/system/` timer and service units without leaving invalid unit syntax or broken state?

## Answer

Ticket resolved through 3 rounds of design tree grilling.

### 1. Canonical Manifest Specification (`arkon.yaml`)

The manifest is versioned (`version: "1"`) and structured into modular, open-for-extension sections:

```yaml
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0"
  power_action: "poweroff" # enum: poweroff (ACPI S5), suspend (ACPI S3), hibernate (ACPI S4)

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
  pre_sleep: []
  post_wake: []

inhibition:
  default_duration: "2h"
  daytime_protection: true
```

* **Hooks Lifecycle:** Supports explicit stages (`pre_teardown`, `post_teardown`, `pre_sleep`, `post_wake`) with `name`, `command`, `timeout_seconds`, and `ignore_errors`, while allowing bare string command shorthand.
* **Manifest Discovery Hierarchy:**
  1. `--config <path>` flag
  2. `./arkon.yaml` or `./arkon.yml`
  3. `/etc/arkon/arkon.yaml` or `/etc/arkon/arkon.yml`

### 2. Native Systemd Unit Compiler & Atomic Installer (`arkon apply`)

* **Unit Pair:** Generates `/etc/systemd/system/arkon-sleep.timer` and `/etc/systemd/system/arkon-sleep.service`.
* **Zero Shutdown Storm:** Timer sets `Persistent=false` so missed sleep windows while the host was offline do not trigger an immediate shutdown upon boot.
* **Service Definition:** `Type=oneshot`, executing `ExecStart=/usr/local/bin/arkon sleep --config <resolved-path>`.
* **Embedded Templates:** Templates are compiled into the Go binary using `embed.FS`, ensuring zero external template file dependencies.
* **Atomic Pre-Commit & Verification:**
  1. Compiles unit files into an isolated temporary directory (`os.CreateTemp`).
  2. Runs syntax verification via Go template validation and `systemd-analyze verify <unit-path>` when available.
  3. Backs up existing Arkon units.
  4. Atomically moves verified units into `/etc/systemd/system/`.
  5. Executes `systemctl daemon-reload` and `systemctl enable --now arkon-sleep.timer`.
  6. Automatically rolls back to the backup units if verification or daemon-reload fails.
* **Privilege Enforcement:** Requires root (`euid == 0`) for filesystem commitment, but provides `--dry-run` to output unit contents and validation diagnostics safely for non-root operators and CI pipelines.

### 3. AI-Agent Native Ergonomics & Contract

* **Introspection:** Built-in `arkon schema` prints the full canonical Draft 2020-12 JSON Schema for manifests and state to stdout, eliminating model hallucination.
* **Machine Contract:** Global `--json` flag formats all command responses into a standardized envelope:
  `{"success": bool, "data": ..., "error": ...}`.
* **Deterministic Exit Codes:**
  - `0`: Success / OK
  - `1`: General Error
  - `2`: Power Transition Inhibited (Active Lock)
  - `3`: Manifest Validation / Compilation Failure
  - `4`: Insufficient Privileges (Root required without `--dry-run`)
