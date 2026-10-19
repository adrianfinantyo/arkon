# Manifest Specification (`arkon.yaml`)

This specification documents the complete configuration schema for `arkon.yaml` (Schema Version 1).

---

## Minimal Example

```yaml
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0/wakealarm"
  power_action: "poweroff"

circadian:
  sleep_at: "00:00"
  wake_at: "08:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]

teardown:
  provider: "docker-compose"
  timeout_seconds: 60
  compose_stacks:
    - path: "/home/adrian/deployment-stack"
  profile_order:
    - "apps"
    - "infra"
```

---

## Schema Sections

### Root Fields

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `version` | string | Yes | Manifest schema version. Must be `"1"`. |
| `host` | object | Yes | Hardware and host power parameters. |
| `circadian` | object | Yes | Daily sleep and wake schedule. |
| `startup` | object | No | Morning boot workload orchestration. |
| `teardown` | object | Yes | Nightly workload teardown pipeline. |
| `hooks` | object | No | Custom lifecycle maintenance scripts. |
| `inhibition` | object | No | Operational protection windows and lock settings. |

---

### `host`

Configures host hardware targets and power transition behavior.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `timezone` | string | `"UTC"` | Valid IANA timezone name (e.g., `"Asia/Jakarta"`, `"Europe/London"`). |
| `rtc_device` | string | `"/sys/class/rtc/rtc0/wakealarm"` | Path to the Linux Real-Time Clock wakealarm sysfs interface. Can also be overridden via the `ARKON_RTC_DEVICE` environment variable. |
| `power_action` | string | `"poweroff"` | Hardware ACPI power transition mode. Allowed values: `"poweroff"` (ACPI S5), `"suspend"` (ACPI S3), `"hibernate"` (ACPI S4). |

---

### `circadian`

Defines the daily operating window for the machine.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `sleep_at` | string | None | Scheduled sleep time in 24-hour `HH:MM` format. |
| `wake_at` | string | None | Scheduled wake time in 24-hour `HH:MM` format. Must maintain at least 180 seconds of safety buffer from `sleep_at`. |
| `days` | array of strings | `["Mon", ... "Sun"]` | Days of the week when circadian schedule triggers. Allowed values: `["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]`. |

---

### `startup`

Orchestrates workload boot when the host powers on.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `provider` | string | `"docker-compose"` | Workload engine provider. Currently supported: `"docker-compose"`. |
| `timeout_seconds` | integer | `120` | Maximum startup timeout in seconds across compose targets. |
| `compose_stacks` | array of objects | `[]` | List of stack directories containing compose files (`path: "/path/to/dir"`). |
| `profiles` | array of strings | `[]` | Sequential profile boot order. If omitted, defaults to reversing `teardown.profile_order`. |

---

### `teardown`

Configures the orderly shutdown pipeline prior to cutting hardware power.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `provider` | string | `"docker-compose"` | Workload engine provider. Currently supported: `"docker-compose"`. |
| `timeout_seconds` | integer | `60` | Maximum shutdown timeout in seconds per profile tier. |
| `compose_stacks` | array of objects | `[]` | List of compose stack directories targeted during teardown (`path: "/path/to/dir"`). |
| `profile_order` | array of strings | `[]` | Strict sequential profile stopping order. Services tagged with the first profile are stopped before subsequent profiles. Untagged residual containers are caught automatically during final cleanup. |

---

### `hooks`

Allows executing arbitrary host shell commands at specific lifecycle checkpoints.

Supported hook lists:
- `pre_teardown`: Executed before any container profiles stop.
- `post_teardown`: Executed after all container tiers and residual stops finish, before filesystem sync.
- `pre_sleep`: Executed immediately before hardware RTC arming and ACPI power transition.
- `post_wake`: Executed during morning boot before starting container stacks.

Each hook entry supports:

| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Descriptive label for status displays and logs. |
| `command` | string | Yes | Shell command line string to execute. |
| `timeout_seconds` | integer | No | Execution timeout in seconds (defaults to 30). |
| `ignore_errors` | boolean | No | If true, non-zero command exit codes log warnings but do not abort the lifecycle sequence. |

---

### `inhibition`

Configures maintenance locks and operational protection.

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `default_duration` | string | `"2h"` | Default time duration for manual inhibition locks when `--duration` is omitted. |
| `daytime_protection` | boolean | `true` | When true, running `arkon sleep` during daytime operational hours aborts with exit code 2 unless overridden with `--force`. |
| `daytime_start` | string | `circadian.wake_at` | 24-hour `HH:MM` start of daytime protected window. |
| `daytime_end` | string | `circadian.sleep_at` | 24-hour `HH:MM` end of daytime protected window. |
