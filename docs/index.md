# Arkon Documentation

Declarative host power lifecycle and pluggable workload teardown orchestrator for Linux homelabs.

Arkon bridges the gap between homelab workloads (Docker Compose, Podman, Kubernetes, systemd slices) and host hardware power management (ACPI S5 soft-off, Real-Time Clock wakealarms, and disk spindowns).

---

## Core Principles

- **Zero Resident Memory (0 MB Idle Heap):** Arkon is a single static Go binary. It compiles declarative manifests into native Linux systemd services and timers, consuming zero RAM while idle.
- **Graceful Workload Teardown:** Databases and stateful containers stop in strict dependency order before filesystem buffers flush to disk (`syscall.Sync()`).
- **Circadian Hardware Power:** Hosts power down into ACPI S5 at night and wake automatically via RTC wakealarms in the morning.
- **Inhibition Locks:** Active backup jobs or photo imports can delay sleep without editing configuration files or cron tabs.
- **Machine-Ready Interface:** Every command supports deterministic exit codes and typed JSON envelopes for scripts and AI agents.

---

## Documentation Roadmap

### Operator Guides
Practical guides for configuring, deploying, and maintaining Arkon on homelab servers:
- [Homelab Deployment Guide](guides/homelab-deployment.md): Complete setup with Docker Compose, GitOps repos, and morning boot orchestration.
- [Hardware RTC & Power Troubleshooting](guides/troubleshooting-hardware.md): Diagnostic runbook for ACPI power states, RTC wakealarm sysfs, and clock drifts.

### Reference Specifications
Complete syntax and machine contracts:
- [CLI & Exit Code Reference](reference/cli.md): All commands, flags, execution privileges, and deterministic exit code contracts.
- [Manifest Specification (`arkon.yaml`)](reference/manifest.md): Schema reference for host settings, circadian schedules, hooks, and teardown profiles.

### Architecture & Extension
Internal architecture and contributor guides:
- [Pluggable Workload Providers](architecture/workload-providers.md): How `TeardownProvider` and `StartupProvider` work, and how to add new engines.
- [Architectural Decision Records (ADRs)](adr/0001-git-hooks-and-ci-parity.md): Accepted technical decisions, including Git hooks, binary distribution, and active host authority.
