# Homelab Deployment Guide

This guide walks through deploying Arkon on a single-node Linux homelab server running Docker Compose stacks. It covers manifest parameterization, promoting host authority, morning boot orchestration, and managing maintenance windows.

---

## 1. Prerequisites & Architecture

Arkon manages power transitions for the whole host. A standard homelab setup typically consists of:
- A Linux host running Debian, Ubuntu, or Alpine with systemd and ACPI support.
- A local deployment directory (for example, `/home/adrian/deployment-stack`) containing your `docker-compose.yaml` files and environment configs.
- Container profiles separating stateless applications from stateful storage layers (such as `proxy`, `apps`, and `infra`).

Arkon does not replace Docker Compose or systemd. Instead, it compiles your operational policy into native systemd units that trigger commands at configured hours.

---

## 2. Parameterizing `arkon.yaml`

Place an `arkon.yaml` file inside your deployment stack directory:

```yaml
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0/wakealarm"
  power_action: "poweroff" # ACPI S5 soft-off

circadian:
  sleep_at: "00:00"
  wake_at: "08:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]

startup:
  provider: "docker-compose"
  timeout_seconds: 120
  compose_stacks:
    - path: "/home/adrian/deployment-stack"
  profiles:
    - "proxy"
    - "infra"
    - "apps"

teardown:
  provider: "docker-compose"
  timeout_seconds: 60
  compose_stacks:
    - path: "/home/adrian/deployment-stack"
  profile_order:
    - "apps"     # 1. Stop web applications (Nextcloud, n8n)
    - "infra"    # 2. Stop databases (PostgreSQL, Redis) so WAL flushes
    - "proxy"    # 3. Stop reverse proxy and tunnels last

hooks:
  post_wake:
    - name: "isolate-failing-seagate-hdd"
      command: "/home/adrian/deployment-stack/scripts/manage-failing-hdd.sh disable"
      timeout_seconds: 15
      ignore_errors: true

inhibition:
  default_duration: "2h"
  daytime_protection: true
```

### Why Profile Ordering Matters
When power drops unexpectedly, databases risk corrupted transaction logs. By defining `profile_order`:
1. Arkon stops customer-facing `apps` first, rejecting new writes.
2. It stops stateful `infra` (PostgreSQL, Redis) second, allowing clean flushes.
3. It runs `syscall.Sync()` and polls `/proc/meminfo` until dirty filesystem cache drains to disk before touching hardware power.

---

## 3. Promoting Active Host Authority

Once your manifest is configured, run `arkon apply` with root privileges from your stack directory:

```bash
cd /home/adrian/deployment-stack
sudo arkon apply
```

Arkon performs five atomic steps:
1. Validates the manifest schema and checks systemd syntax.
2. Promotes `/home/adrian/deployment-stack/arkon.yaml` to the active host authority.
3. Establishes the canonical symlink `/etc/arkon/arkon.yaml` pointing to your stack manifest.
4. Records state metadata in `/var/lib/arkon/state.json`.
5. Compiles and installs two native systemd units:
   - `arkon-sleep.timer` + `arkon-sleep.service`: Triggers `arkon sleep` at midnight.
   - `arkon-boot.service`: Triggers `arkon up` on system boot via `multi-user.target`.

### Inspecting Installation Without Modifying the Host
You can preview the generated unit files without root or disk modifications:

```bash
arkon apply --dry-run
```

---

## 4. Morning Wake & Boot Orchestration

When the motherboard RTC triggers wakealarm at 08:00 AM, the host boots into Linux. Once systemd reaches `multi-user.target`:
1. `arkon-boot.service` executes `arkon up`.
2. Any configured `post_wake` hooks execute first (for example, unmounting or detaching degraded mechanical storage drives).
3. Containers start in the declared profile order (`proxy`, `infra`, `apps`).

You can manually trigger the boot sequence at any time for verification:

```bash
sudo arkon up
```

---

## 5. Routine Status Checks

Any non-root user can inspect full host status from any directory:

```bash
arkon status
```

Arkon discovers the active host manifest via `/etc/arkon/arkon.yaml` and displays:
- Host power state and CPU architecture.
- Circadian schedule and time until next sleep.
- Hardware RTC wakealarm countdown and clock mode.
- Startup and teardown pipelines.
- Configured lifecycle hooks.
- Active maintenance locks.

If you edit your local stack manifest without running `sudo arkon apply`, `arkon status` automatically detects the divergence and displays a clear `● Active Manifest Notice` indicating manifest drift.

---

## 6. Managing Maintenance Windows

If a long-running job (like an offsite Restic backup or bulk photo indexing) is running past midnight, prevent nighttime shutdown by acquiring an inhibition lock:

```bash
# Inhibit sleep for 3 hours
arkon inhibit --reason "Restic offsite backup" --duration 3h
```

Arkon records the lock in `/var/lib/arkon/state.json`. If `arkon sleep` executes while a lock is active, it aborts with exit code 2 and leaves the server running. Once the duration expires, normal circadian scheduling resumes automatically.
