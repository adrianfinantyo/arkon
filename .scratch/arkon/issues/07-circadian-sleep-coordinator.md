# 07: Circadian Sleep Coordinator (`arkon sleep`)

**What to build:** The primary power transition orchestrator executed by the nightly systemd service or manual operator invocation. Coordinates daytime checks, active lock evaluations, lifecycle hooks, phased container profile teardown, filesystem buffer drain, hardware RTC wakealarm programming, and host poweroff.

**Blocked by:** 02: Manifest Schema, Discovery Hierarchy & Validation Engine; 04: Inhibition Locks, Daytime Protection & State Engine (`arkon inhibit`); 05: Hardware RTC Wakealarm Driver & ACPI Subsystem Abstraction; 06: Profile-Aware Workload Teardown Pipeline & Filesystem Buffer Sync

**Status:** resolved

- [x] Evaluates the daytime operational protection window; aborts with exit code 2 if currently within daytime hours, unless `--force` is specified.
- [x] Checks active inhibition locks from state; aborts with exit code 2 and outputs active lock details if a valid lock is present, unless `--force` is specified.
- [x] Executes `pre_sleep` lifecycle hooks in order.
- [x] Invokes the workload teardown pipeline across all configured tiers, followed by residual container stopping and dirty page flushing.
- [x] Computes next scheduled wake timestamp from circadian policy and programs the hardware RTC wakealarm.
- [x] Enforces safety checks: verifies RTC wakealarm retention before initiating the configured host power action.
- [x] Executes the final host power transition (`systemctl poweroff`, `suspend`, or `hibernate`).
- [x] Supports `--dry-run` to simulate the full sleep sequence without stopping containers, writing RTC sysfs, or cutting host power.
