## Destination

A complete architectural blueprint and technical specification for **Arkon**, detailing the declarative manifest schema (`arkon.yaml`), the systemd unit compiler & installer (`arkon apply`), the low-level Linux RTC wakealarm & ACPI S5 subsystem abstraction, the profile-aware Docker Compose container teardown engine, and the ergonomic terminal CLI interface (`status`, `sleep`, `inhibit`), fully prepared for implementation as a zero-dependency static Go binary.

## Notes

- Domain: Host power lifecycle, Docker Compose orchestration, Linux ACPI/RTC hardware alarms, low-power homelab appliances.
- Consult skills: `grilling`, `domain-modeling`, `codebase-design`.
- Read vocabulary: [`GLOSSARY.md`](file:///home/adrian/adrian-dev/arkon/GLOSSARY.md).
- Hardware constraints: Intel Pentium Dual-Core E5400 (2C/2T @ 2.7GHz), 3.8 GiB RAM, 256GB SSD, mechanical storage HDDs.
- Key invariants:
  - CLI-first architecture: no persistent web daemon or background memory footprint.
  - GitOps-driven: `arkon.yaml` is the single source of truth for host lifecycle parameters.
  - Zero ungraceful database kills: strict multi-tier container stopping order with filesystem sync.
  - Pluggable hooks: server-specific commands (SCSI detach, database dumps) remain external.

## Decisions so far

<!-- the index: one line per closed ticket, enough to judge relevance, then zoom the link for the detail the ticket holds -->
- [[01] Declarative Manifest Schema & Systemd Compiler](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/wayfinder/01-manifest-schema-and-compiler.md): Version 1 manifest schema with modular lifecycle hooks, host power action abstraction, AI schema introspection ('arkon schema'), deterministic '--json' contract, and atomic systemd compiler with verified rollback and 'Persistent=false' timer.
- [[02] Linux RTC Wake & ACPI S5 Abstraction](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/wayfinder/02-rtc-wake-and-acpi-s5-abstraction.md): Program hardware wakealarm via direct sysfs writes (`/sys/class/rtc/rtc0/wakealarm`) using relative syntax (`+<seconds>`) with pre-clearing write (`0\n`); eliminate `rtcwake` subprocess; handle `/etc/adjtime` clock modes for status inspection.
- [[03] Profile-Aware Container Teardown Engine](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/wayfinder/03-profile-aware-container-teardown.md): Pluggable 'TeardownProvider' seam (Docker Compose default, Podman, command/PM2), phased sequential profile stopping, residual catch-all, mandatory 'syscall.Sync()' with dirty page drain, and fail-safe abort.
- [[04] Inhibition Locks & State Management](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/wayfinder/04-inhibition-locks-and-state-management.md): Atomic Root SSD state persistence ('/var/lib/arkon/state.json') guarded by 'flock', auto-expiring timed inhibition locks, daytime operational window protection with explicit violation messages, and exit code 2 machine contract.
- [[05] CLI Command Surface & Terminal UX](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/wayfinder/05-cli-command-surface-and-terminal-ux.md): Single unified borderless left-rail layout (immune to terminal width/wrapping, zero dual-view bloat), actionable diagnostic error reporting, and deterministic '--json' machine contract.

## Active tickets

- [[09] Host Environment Seam Integrity & CLI Runner Decomposition](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/09-host-environment-seam-and-runner-decomposition.md)
- [[10] State File Corruption Resilience & Recovery](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/10-state-file-corruption-resilience.md)
- [[11] Spec Reconciliation & Daytime Window Configuration](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/11-spec-reconciliation-and-daytime-window-configuration.md)
- [[12] Test Coverage Hardening & Binary-Level Exit Code Contracts](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/12-test-coverage-hardening-and-binary-contracts.md)

## Not yet specified

- **Automated Update & Binary Distribution**: Packaging Arkon as Debian packages (`.deb`) and standalone GitHub release binaries via GoReleaser.
- **Healthcheck & Boot Diagnostics**: Post-boot verification commands testing network connectivity and container health before marking the circadian wake cycle successful.

## Out of scope

- Heavy persistent web UI or background HTTP daemon (monitoring is handled by Beszel).
- Cloud orchestration, multi-node clustering, or Kubernetes integration (Arkon is single-node/homelab-first).
- Direct block-level filesystem formatting or volume partitioning.
