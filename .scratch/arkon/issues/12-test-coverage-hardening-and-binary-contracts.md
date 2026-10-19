# 12: Test Coverage Hardening & Binary-Level Exit Code Contracts

**What to build:** Close remaining test coverage gaps identified in PR #14 code review. Add dedicated test scenarios for systemd installer atomic rollback on `daemon-reload` or filesystem write failure, hook timeout enforcement via context deadline cancellation, and end-to-end binary execution verifying exit codes 2, 3, and 4 in `cmd/arkon/main_test.go`.

**Blocked by:** 09: Host Environment Seam Integrity & CLI Runner Decomposition, 10: State File Corruption Resilience & Recovery

**Status:** resolved

- [x] Write unit test in `internal/systemd/systemd_test.go` verifying that if `systemctl daemon-reload` returns an error, the installer restores backed-up unit files and issues a recovery reload.
- [x] Write unit test in `internal/systemd/systemd_test.go` verifying that if destination file write fails during installation, existing units are not corrupted and backups are restored.
- [x] Write unit test in `internal/teardown/coordinator_test.go` verifying that hooks running beyond their configured `timeout_seconds` are canceled via context deadline.
- [x] Expand `cmd/arkon/main_test.go` to test binary-level execution for all deterministic exit codes:
  - Exit code 2 (inhibited transition via active lock or daytime operational window)
  - Exit code 3 (manifest validation or compilation failure)
  - Exit code 4 (insufficient privileges when running `apply` without root and without `--dry-run`)
- [x] All test suites pass cleanly across all packages.
