# Arkon

Declarative host power lifecycle and pluggable workload teardown orchestrator for Linux homelabs (Docker, Podman, K8s, systemd).

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8.svg?logo=go)](go.mod)
[![Platform](https://img.shields.io/badge/Platform-Linux%20(amd64%20%7C%20arm64)-orange.svg)](docs/)

Arkon bridges the gap between **homelab workloads (Docker Compose, Podman, K8s/K3s, systemd slices, or custom CLI commands)** and **host hardware power management** (ACPI S5, Real-Time Clock wakealarms, systemd units, and disk spindowns).

It is built as the infrastructure sibling to [Arkive](https://github.com/adrianfinantyo/arkive) (lightweight media archive and file sharing), sharing the same philosophy: **build software that treats older or modest hardware with respect.**

---

## The Problem

I run several homelab machines, including an older desktop PC (Intel Pentium Dual-Core E5400) with mechanical storage hard drives. Running these machines 24/7 became an issue in practice:

1. **24/7 power is wasteful and noisy:** An idle 45W machine running around the clock consumes ~394 kWh/year. Putting it to sleep from midnight to 07:00 AM saves ~115 kWh/year, eliminates nighttime noise, and reduces wear on mechanical hard drive spindles.
2. **Standard Linux power tools are blunt instruments:**
   * Running `rtcwake -m mem` or raw cron `shutdown -h` cuts power directly out from under running containers. Databases like PostgreSQL, Redis, and SQLite risk corrupted write-ahead logs or unfinished writes.
   * Dirty filesystem buffers are not guaranteed to flush cleanly to mechanical drives before power drops.
3. **Ad-hoc bash scripts rot:**
   * Cron jobs and one-off scripts in `/usr/local/bin` are invisible, unversioned, and break silently when container names change.
4. **Heavy web management panels consume too much RAM:**
   * Container management panels (Portainer, Cockpit, Coolify) consume 150MB to 500MB of resident RAM just sitting idle. On a server with 4GB of RAM, that memory belongs to your applications. Furthermore, web UIs cannot safely program hardware RTC wake alarms or survive Docker daemon reboots during host sleep.

---

## How Arkon Works

Arkon adheres to three core design rules:

### 1. Zero Resident Memory (0 MB Idle Heap)
Arkon is a single static Go binary that acts as a **compiler and control CLI**, not a continuous background daemon.
* You define your host schedule and container teardown order in a declarative `arkon.yaml`.
* Running `arkon apply` compiles that policy directly into native Linux **systemd services and timers**.
* When idle, Arkon consumes **0 MB of RAM** because systemd (PID 1) already handles execution. Arkon only runs during state transitions.

### 2. Pluggable Workload Teardown & Buffer Draining
Workload teardown is decoupled behind the `TeardownProvider` interface to support diverse homelab stacks:
* **Docker Compose:** Phased profile stops (`workers` -> `web` -> `storage`) without bulky embedded parsers.
* **Podman Compose:** Identical profile semantics for rootless and daemonless container hosts.
* **Kubernetes / K3s:** Node cordon and graceful pod draining (`kubectl drain`) before host sleep.
* **Systemd Targets:** Deactivating specific systemd target slices (`systemctl stop homelab.target`).
* **Custom Command Hooks:** Arbitrary CLI commands for PM2 processes, raw binaries, or storage scripts.

Before cutting power or entering ACPI S5:
1. Arkon executes your configured provider to drain background workers and reject incoming traffic.
2. It stops databases (PostgreSQL, Valkey, SQLite) cleanly so write-ahead logs close.
3. It invokes `syscall.Sync()` and verifies `/proc/meminfo` dirty pages drain to zero before power drops.
4. It programs the hardware wake timestamp into `/sys/class/rtc/rtc0/wakealarm`.
5. Only then does it trigger ACPI poweroff or sleep.

### 3. Ergonomic Inhibition Locks
If a long-running backup or photo import is in progress when the shutdown hour arrives:
* Applications or admin scripts call `arkon inhibit --reason "Arkive import running"`.
* Arkon defers the scheduled sleep until the lock releases, preventing interrupted jobs.

### 4. Agent-Ready & Machine-Parsable
All CLI commands support `--json` output, batch-safe execution, and clean exit codes, making it easy for automation scripts and AI agents to manage host state without terminal parsing hacks.

---

## Installation

### 1. Standalone Installer (Recommended)

Install the latest binary directly to `/usr/local/bin/arkon`:

```bash
curl -fsSL https://raw.githubusercontent.com/adrianfinantyo/arkon/main/scripts/install.sh | sudo bash
```

### 2. Debian / Ubuntu Package (`.deb`)

Download the `.deb` asset from [GitHub Releases](https://github.com/adrianfinantyo/arkon/releases) and install via `dpkg`:

```bash
sudo dpkg -i arkon_*_linux_amd64.deb   # on x86_64
sudo dpkg -i arkon_*_linux_arm64.deb   # on aarch64
```

### 3. Build From Source

```bash
git clone https://github.com/adrianfinantyo/arkon.git
cd arkon
go build -ldflags="-s -w" -o bin/arkon ./cmd/arkon
sudo cp bin/arkon /usr/local/bin/arkon
```

---

## Quickstart

### 1. Define your manifest (`arkon.yaml`)

Create `/etc/arkon/arkon.yaml` (or `./arkon.yaml` in your compose directory):

```yaml
version: "1"

host:
  timezone: "Asia/Jakarta"
  rtc_device: "/sys/class/rtc/rtc0/wakealarm"
  power_action: "poweroff" # enum: poweroff (ACPI S5), suspend (ACPI S3), hibernate (ACPI S4)

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
    - "apps"
    - "infra"
    - "proxy"

hooks:
  post_wake:
    - name: "detach-failing-drive"
      command: "/home/adrian/deployment-stack/scripts/manage-failing-hdd.sh disable"
      timeout_seconds: 15
      ignore_errors: true

inhibition:
  default_duration: "2h"
  daytime_protection: true
```

*A fully annotated reference configuration is available at [`arkon.example.yaml`](arkon.example.yaml).*

### 2. Compile and apply systemd units

```bash
sudo arkon apply
```

Running `arkon apply` promotes your manifest to the active host manifest authority:
* It establishes a canonical link at `/etc/arkon/arkon.yaml` and records state in `/var/lib/arkon/state.json`.
* It compiles the declarative policy into native systemd units, validates unit syntax, and enables them atomically:
  * `arkon-sleep.service` + `arkon-sleep.timer`: Runs nightly shutdown (`arkon sleep`).
  * `arkon-boot.service`: Runs morning wake orchestration (`arkon up`), executing post-wake hooks before starting container profiles.

Once applied, `arkon status` and systemd units discover and monitor that host manifest system-wide from any directory.

Inspect units safely without root:
```bash
arkon apply --dry-run
```

### 3. Inspect status

```bash
arkon status
```

Output:
```text
● ARKON HOST STATUS [ACPI S0 (Operational Active)]
  Host:        srv-komsos (Pentium(R) Dual-Core CPU E5400 @ 2.70GHz (x86_64))
  Timezone:    Asia/Jakarta (WIB, UTC+07:00)

● Circadian Schedule (arkon-sleep.timer active)
  Schedule:    Sleep @ 00:00  |  Wake @ 08:00
  Next Action: Sleep (ACPI S5 Soft-Off), Tomorrow at 00:00:00 WIB (in 5h 46m)

● Hardware RTC Wakealarm [DISABLED]
  Device:      /sys/class/rtc/rtc0/wakealarm
  Target:      None scheduled
  Clock Mode:  /etc/adjtime (UTC)
  Kernel Dirty: 576 kB (flushed to disk)

● Workload Startup Pipeline (arkon-boot.service enabled)
  Provider:    docker-compose (1 stack)
  Progression: post_wake hooks → proxy → infra → apps
  Stacks:      /home/adrian/deployment-stack

● Workload Teardown Pipeline
  Provider:    docker-compose (1 stack)
  Progression: apps → infra → proxy → syscall.Sync()
  Stacks:      /home/adrian/deployment-stack

● Lifecycle Hooks
  Post-Wake:   detach-failing-drive

● Maintenance Locks
  Inhibitions: None active (sleep will trigger as scheduled)
```

### 4. Take a temporary maintenance lock

Prevent scheduled nighttime sleep while running backups or imports:

```bash
arkon inhibit --reason "Restic offsite backup" --duration 2h
```

---

## Pluggable Workload Providers & Roadmap

Arkon uses an interface-driven design. Workload engines implement two clean seams:
* `TeardownProvider`: Ordered tier stopping (`StopTier`) and residual catch-all (`StopResidual`).
* `StartupProvider`: Ordered stack start (`StartStack`) across configured profiles.

See [`docs/architecture/workload-providers.md`](docs/architecture/workload-providers.md) for interface specifications and implementation guides.

| Workload Provider | Status | Target Use Case |
| :--- | :--- | :--- |
| **Docker Compose** | **Supported (v0.1.0)** | Standard homelab deployments using profile-tagged compose stacks. |
| **Podman Compose** | In Roadmap | Rootless, daemonless container hosts ([Issue #21](https://github.com/adrianfinantyo/arkon/issues/21)). |
| **Kubernetes / K3s** | In Roadmap | Node cordon and graceful pod draining (`kubectl drain`) ([Issue #22](https://github.com/adrianfinantyo/arkon/issues/22)). |
| **Systemd Slices** | In Roadmap | Deactivating host systemd target slices (`systemctl stop homelab.target`) ([Issue #23](https://github.com/adrianfinantyo/arkon/issues/23)). |

Contributions are welcome! If you run Podman, K3s, or custom workload engines in your homelab, please see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Hardware Compatibility

Arkon interacts directly with standard Linux kernel interfaces:
* **ACPI States:** ACPI S5 (Soft Off / Poweroff), S3 (Suspend to RAM), S4 (Suspend to Disk / Hibernate).
* **RTC Wakealarm:** Standard Linux RTC subsystem (`/sys/class/rtc/rtc0/wakealarm`).
* **Architectures:** Tested on Linux amd64 (Intel Core / Pentium / Xeon, AMD Ryzen / EPYC) and arm64 (Raspberry Pi with RTC hat, Odroid, Rockchip).

---

## Sibling Projects

* [Arkive](https://github.com/adrianfinantyo/arkive): Self-hosted media archive and file sharing designed for small teams and homelab users, built to keep mechanical hard drives spun down during routine browsing.

---

## Contributing

Contributions are welcome! Please check out [CONTRIBUTING.md](CONTRIBUTING.md) for local development setup, testing guidelines, and hardware compatibility reports.

---

## License

[MIT](LICENSE) (c) 2026 Adrian Finantyo

