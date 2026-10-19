# Arkon

A lightweight, single-binary host lifecycle orchestrator and workload controller (Docker, Podman, K8s, systemd) designed for low-power servers, homelabs, and edge appliances.

## Language

**Arkon**:
The single-binary CLI tool and orchestrator managing host power transitions, systemd units, and pluggable workload profiles.
_Avoid_: Control plane, management agent, host orchestrator, Aktive

**Circadian Schedule**:
The automated daily cycle transitioning the physical host between active operational hours and hardware ACPI S5 soft-off.
_Avoid_: Sleep timer, power routine, shutdown cron

**Teardown Pipeline**:
The ordered sequence of operations halting application containers, flushing database caches to disk, and syncing filesystem journals before cutting system power.
_Avoid_: Shutdown hook, stop sequence, kill script

**Teardown Provider**:
The pluggable driver interface orchestrating workload stopping across distinct runtime environments (Docker Compose, Podman Compose, or system commands).
_Avoid_: Container backend, stop plugin, runner

**Wake Alarm**:
The hardware Real-Time Clock alarm programmed via the Linux RTC subsystem to power on the system from ACPI state S5.
_Avoid_: Wake timer, BIOS alarm, power-on timer

**Inhibition Lock**:
A temporary, timed override that prevents scheduled automated shutdowns during critical maintenance or extended workloads.
_Avoid_: Shutdown blocker, sleep disable, pause flag

**Manifest**:
The declarative YAML file (`arkon.yaml`) defining stack profiles, schedules, timeouts, and lifecycle hooks as a single source of truth.
_Avoid_: Config file, settings script, setup spec

**Active Host Manifest**:
The authoritative manifest committed to the host system by `arkon apply` that governs systemd units, RTC wakealarms, and lifecycle status across the entire machine.
_Avoid_: Local config, directory manifest, active profile, current yaml
