# 06: Profile-Aware Workload Teardown Pipeline & Filesystem Buffer Sync

**What to build:** The pluggable `TeardownProvider` interface and Docker Compose driver orchestrating ordered container stopping and filesystem page cache flushing. Ensures databases and stateful containers receive graceful stop signals in configured dependency tiers before power drops, and verifies filesystem write buffers drain to disk.

**Blocked by:** 02: Manifest Schema, Discovery Hierarchy & Validation Engine

**Status:** resolved

- [x] Implements the `TeardownProvider` interface for Docker Compose stacks, executing `docker compose --profile <tier> stop` with per-tier timeout handling.
- [x] Stops containers in sequential profile order (such as `apps` followed by `infra`) as configured in `teardown.profile_order`.
- [x] Executes a residual catch-all stop to gracefully halt untagged containers across configured stacks.
- [x] Executes configured `pre_teardown` and `post_teardown` lifecycle hooks in sequence, honoring individual hook timeouts and `ignore_errors` flags.
- [x] Invokes `syscall.Sync()` to flush dirty file cache buffers to physical storage disks.
- [x] Polls `/proc/meminfo` dirty page counts with a bounded timeout, verifying that dirty bytes drain before proceeding toward poweroff.
