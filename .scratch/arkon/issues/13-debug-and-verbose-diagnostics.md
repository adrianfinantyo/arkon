# 13: Debug and Verbose Diagnostics

**What to build:** Add `--verbose` and `--debug` flags to the CLI to provide actionable diagnostics during teardown, container stopping, filesystem cache flushing, and hardware RTC operations. When enabled, print step execution timings, individual container stop responses, dirty page drain measurements, and raw sysfs read and write values to standard error without corrupting deterministic JSON contracts.

**Blocked by:** 09: Host Environment Seam Integrity & CLI Runner Decomposition

**Status:** ready-for-agent

- [ ] Add `--verbose` and `--debug` flags to CLI flag parsing in `internal/cli`.
- [ ] Propagate verbosity configuration to `lifecycle.Coordinator`, `teardown.Coordinator`, and `rtc.Manager`.
- [ ] Emit detailed diagnostic logs to standard error during teardown: individual container profiles and names stopped, elapsed durations per tier, and dirty page counts polled from `/proc/meminfo`.
- [ ] Emit detailed diagnostic logs to standard error during RTC wakealarm operations: sysfs path, reset write, programmed epoch, and readback verification.
- [ ] Ensure `--json` output to standard output remains clean and parseable when `--verbose` or `--debug` is active.
- [ ] Unit tests in `internal/cli/` and relevant packages verify diagnostic output formatting and JSON isolation.
