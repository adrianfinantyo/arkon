# 03: Native Systemd Unit Compiler & Atomic Installer (`arkon apply`)

**What to build:** The compiler and atomic installer that generates native `/etc/systemd/system/arkon-sleep.service` and `/etc/systemd/system/arkon-sleep.timer` unit files from `arkon.yaml`. Operators can test and preview generated unit syntax without root privileges via `--dry-run`, or commit verified units atomically with automatic rollback if systemd verification fails.

**Blocked by:** 02: Manifest Schema, Discovery Hierarchy & Validation Engine

**Status:** resolved

- [x] Generates `arkon-sleep.service` as `Type=oneshot` executing `arkon sleep --config <resolved-path>`.
- [x] Generates `arkon-sleep.timer` with `Persistent=false` and calendar triggers derived from manifest `circadian` configuration.
- [x] Compiles units using templates embedded into the binary via `embed.FS` with zero external file dependencies.
- [x] Running `arkon apply --dry-run` compiles units into memory, verifies unit syntax, and prints generated unit contents to stdout without writing files or requiring root privileges.
- [x] Running `arkon apply` without root privileges (EUID != 0) aborts with exit code 4 unless `--dry-run` is passed.
- [x] Atomically stages unit files in a temporary directory, backs up existing units, installs new units to `/etc/systemd/system/`, and reloads systemd via `systemctl daemon-reload`.
- [x] Automatically rolls back to backed-up units if filesystem writes or `daemon-reload` execution encounters an error.
