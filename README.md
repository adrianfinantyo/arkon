# Arkon

**Arkon** is an ultra-lightweight, single-binary CLI tool and host lifecycle orchestrator built for homelabs, energy-conscious self-hosters, and edge appliances.

It bridges the gap between **Docker Compose stack profiles** and **host hardware power management** (ACPI S5, Real-Time Clock wakealarms, systemd units, and disk protection).

---

## The Problem

Most self-hosters run small appliances (Intel NUCs, vintage office PCs, refurbished desktops, NAS machines) in living spaces or home offices. Running them 24/7 generates unnecessary heat, noise, energy bills, and mechanical drive platter wear.

However, scheduled power cycling is notoriously fragile:
* Standard cron and `rtcwake` scripts kill containers ungracefully, corrupting databases (PostgreSQL/Redis/SQLite).
* Shutdowns and wake timers are buried in invisible host files (`/etc/crontab`, `/etc/systemd/system/`, `/usr/local/bin/`).
* Heavy container management web UIs (Portainer, Cockpit, Coolify) consume valuable RAM and cannot manipulate hardware RTC alarms or survive Docker daemon restarts.

---

## The Arkon Solution

Arkon turns host lifecycle management into an ergonomic, GitOps-ready experience:

* **Declarative Manifest (`arkon.yaml`):** Declare your shutdown schedule, target wake time, and profile teardown order in a tracked YAML file.
* **`arkon apply`:** Compiles your manifest into clean, native systemd service and timer units.
* **`arkon status`:** Instantly inspect current power state, timer health, and next scheduled wake alarm.
* **`arkon sleep` & `arkon inhibit`:** Manually trigger clean, dependency-ordered teardown or temporarily pause scheduled shutdowns during backups.
* **Pluggable Lifecycle Hooks:** Run pre-shutdown backups, detach failing SCSI drives, and flush dirty buffers automatically.
* **Ultra-Low Footprint:** Written in Go as a static binary with no background daemons, zero external dependencies, and negligible memory overhead.

---

## License

MIT
