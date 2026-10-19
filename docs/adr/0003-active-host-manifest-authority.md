# ADR 0003: Active Host Manifest Authority and Discovery Architecture

- **Status:** accepted
- **Date:** 2026-10-19
- **Deciders:** Adrian Finantyo

## Context

Arkon was originally designed with directory-local manifest discovery inspired by container orchestration tools like Docker Compose. When invoked, Arkon searched for `./arkon.yaml` in the current working directory before falling back to `/etc/arkon/arkon.yaml`.

In practice, this led to operator confusion and potential split-brain risks:
1. **Single Physical Host Lifecycle:** A physical machine possesses exactly one ACPI S5 power state, one hardware Real-Time Clock wakealarm device (`/sys/class/rtc/rtc0/wakealarm`), and one systemd supervisor (`systemd` PID 1). Multiple manifests cannot govern the physical host simultaneously.
2. **Silent Unconfigured Fallback:** When an operator logged into a host via SSH (landing in `/home/user`) and executed `arkon status`, Arkon looked for `./arkon.yaml` in `/home/user`. Because no manifest existed there, `arkon status` silently fell back to an unconfigured state, reporting `Schedule: Disabled` and 0 stacks, even though systemd units were already actively managing a stack located in `/home/user/deployment-stack`.
3. **Drift Risk:** If multiple directories contained distinct `arkon.yaml` files, executing `arkon sleep` from one directory could shut down the host using different profile teardown policies than the systemd timer registered during `arkon apply`.

## Decision

We establish the **Active Host Manifest** as the single source of truth for the physical machine.

### 1. Active Manifest Registration on `arkon apply`

When `sudo arkon apply` runs:
1. **Source Manifest Promotion:** The manifest passed via `-c <path>` (or resolved from `./arkon.yaml` in CWD) is registered as the machine's Active Host Manifest.
2. **Canonical Host Symlink:** Arkon manages `/etc/arkon/arkon.yaml` as a symlink pointing to the absolute path of the source manifest. If `/etc/arkon` does not exist, Arkon creates it with `0755` permissions so non-root users can read it.
3. **Host State Persistence:** The absolute path of the active manifest, the applied timestamp, and the applying user are persisted in root state (`/var/lib/arkon/state.json`).
4. **Systemd Unit Parameterization:** Systemd units (`arkon-sleep.service`, `arkon-boot.service`) are compiled with `--config /etc/arkon/arkon.yaml`, referencing the canonical path.

### 2. Manifest Discovery Order

All operational CLI commands (`status`, `sleep`, `inhibit`, `up`, `wake`) resolve their configuration in the following order:
1. **Explicit Flag (`-c / --config <path>`):** Always takes highest precedence for intentional overrides or manual dry-runs.
2. **Active Host Manifest:** Resolved via `/var/lib/arkon/state.json` or `/etc/arkon/arkon.yaml`.
3. **Directory-Local Fallback:** If no active manifest is registered on the host, Arkon checks `./arkon.yaml` in the current working directory.

### 3. Local Drift Detection

If an operator executes `arkon status` while located in a directory containing an unapplied `./arkon.yaml` whose real path differs from the Active Host Manifest:
- Arkon reports the status of the **Active Host Manifest** (reflecting true host state).
- Arkon prints a notice informing the operator that an unapplied local manifest was detected, advising them to run `sudo arkon apply` to activate it or pass `-c ./arkon.yaml` to inspect it.

### 4. `arkon apply` Source Resolution

When running `sudo arkon apply` without `-c`:
1. If `./arkon.yaml` exists in the current working directory, Arkon applies it and promotes it to the Active Host Manifest.
2. If no `./arkon.yaml` exists in CWD, but an Active Host Manifest is already registered on the host, Arkon re-synchronizes the host units using the existing active manifest.
3. If neither exists, Arkon aborts with exit code 3 and instructions to specify a manifest path.

### 5. Unconfigured and Degraded Status Contracts

- **Unconfigured Host:** When no manifest has been registered and `/etc/arkon/arkon.yaml` does not exist, `arkon status` explicitly displays `● Circadian Schedule [NOT CONFIGURED]` with instructions on how to apply a manifest, rather than silently showing empty defaults.
- **Broken Active Manifest:** If the file referenced by the Active Host Manifest is deleted or moved, `arkon status` displays `● Circadian Schedule [BROKEN ACTIVE MANIFEST]` with the missing path. Automated sleep runs abort cleanly to protect the host from running an uncontrolled power transition.

## Consequences

### Positive
- Predictable behavior across all working directories. Running `arkon status` from `~`, `/tmp`, or a project directory displays the true host schedule.
- Zero split-brain configurations between interactive CLI commands and automated systemd timers.
- Clean GitOps compatibility: operators can keep `arkon.yaml` in version-controlled stack directories, and `arkon apply` links it to system authority.
- Clear diagnostic warnings when working in directories with unapplied manifest drafts.

### Negative
- `arkon apply` requires write access to `/etc/arkon` and `/var/lib/arkon`, which requires root privileges (already required to install systemd units).
