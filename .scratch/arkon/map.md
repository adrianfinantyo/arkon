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
- [[09-13] Core Seam Hardening, Spec Reconciliation, Exit Codes & Verbose Diagnostics](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/): Resolved PR #14 review findings across HostEnvironment seam integrity, state corruption quarantine recovery, configurable daytime windows, binary-level exit code assertions (codes 2, 3, 4), and verbose diagnostic logging to stderr.
- [[14] Catch-All Workload Teardown via Profile Wildcard](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/14-compose-stop-all-profiles.md): Pass '--profile "*"' in StopResidual to ensure untracked or profile-tagged containers across any profile (such as proxy services) are stopped cleanly before poweroff.
- [[15] Declarative Startup Orchestrator & Boot Systemd Unit](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/15-startup-orchestration-and-boot-unit.md): Add declarative 'startup:' manifest block, 'arkon up' CLI command for boot sequences, and compile/install 'arkon-boot.service' on multi-user.target to unify full boot-to-shutdown operations into arkon.yaml.

## Active tickets

- [[16] Status Command Startup & Hooks Visibility](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/16-status-startup-and-hooks-visibility.md)
- [[17] Local CI Parity & Pre-Push Hook Automation](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/17-git-hooks-ci-parity.md)
- [[18] GoReleaser Packaging & Release Automation](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/18-goreleaser-and-release-automation.md)
- [[19] Workload Provider Extensibility Documentation & Roadmap](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/19-workload-provider-extensibility-docs.md)
- [[20] Documentation & Manifest Alignment](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/issues/20-documentation-and-manifest-alignment.md)

## Not yet specified

- **Pluggable Workload Providers (Podman, K8s, systemd slices)**: Extend the workload teardown abstraction beyond Docker Compose to support rootless Podman compose pods, local single-node Kubernetes clusters (e.g., k3s / kubelet drain), and systemd scope/slice units.
- **Automated Update & Binary Distribution**: Packaging Arkon as Debian packages (`.deb`) and standalone GitHub release binaries via GoReleaser.
- **Healthcheck & Boot Diagnostics**: Post-boot verification commands testing network connectivity and container health before marking the circadian wake cycle successful.

## Out of scope

- Heavy persistent web UI or background HTTP daemon (monitoring is handled by Beszel).
- Multi-node clustering or distributed cluster orchestration (Arkon is single-node and homelab-first).
- Direct block-level filesystem formatting or volume partitioning.

