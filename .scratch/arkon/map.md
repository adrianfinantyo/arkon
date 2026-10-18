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

## Active tickets

- [[01] Declarative Manifest Schema & Systemd Compiler](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/01-manifest-schema-and-compiler.md): Defining the specification for `arkon.yaml`, schema validation, and the template compiler generating native `/etc/systemd/system/` timer and service units.
- [[02] Linux RTC Wake & ACPI S5 Abstraction](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/02-rtc-wake-and-acpi-s5-abstraction.md): Native Go kernel interface for `/sys/class/rtc/rtc0/wakealarm` vs `/usr/sbin/rtcwake` execution, local/UTC time conversion, and hardware alarm verification.
- [[03] Profile-Aware Container Teardown Engine](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/03-profile-aware-container-teardown.md): Docker Compose CLI wrapper vs direct Docker Engine API socket client (`/var/run/docker.sock`), dependency profile stopping, timeouts, and filesystem sync coordination.
- [[04] Inhibition Locks & State Management](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/04-inhibition-locks-and-state-management.md): Atomic lockfile and JSON state on the Root SSD (`/var/lib/arkon/state.json`) governing `arkon inhibit`, daytime execution locks, and auto-expiring overrides.
- [[05] CLI Command Surface & Terminal UX](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/05-cli-command-surface-and-terminal-ux.md): Designing command ergonomics, table rendering (`status`), JSON output flags, error semantics, and privilege elevation (`sudo`) handling.

## Not yet specified

- **Automated Update & Binary Distribution**: Packaging Arkon as Debian packages (`.deb`) and standalone GitHub release binaries via GoReleaser.
- **Healthcheck & Boot Diagnostics**: Post-boot verification commands testing network connectivity and container health before marking the circadian wake cycle successful.

## Out of scope

- Heavy persistent web UI or background HTTP daemon (monitoring is handled by Beszel).
- Cloud orchestration, multi-node clustering, or Kubernetes integration (Arkon is single-node/homelab-first).
- Direct block-level filesystem formatting or volume partitioning.
