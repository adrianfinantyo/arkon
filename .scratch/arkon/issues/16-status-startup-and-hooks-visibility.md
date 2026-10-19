# 16: Status Command Startup & Hooks Visibility

**What to build:** Extend the operator status command (`arkon status`) to display both the workload startup pipeline and configured lifecycle hooks in the borderless left-rail layout and the typed JSON envelope (`StatusData`).

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Add `StartupStatus` and `HooksStatus` types to `internal/status/types.go` and embed in `StatusData`.
- [ ] Implement `collectStartup` and `collectHooks` in `internal/status/collector.go`.
- [ ] Inspect `arkon-boot.service` unit enabled state via `hostEnv.Exec(ctx, "systemctl", "is-enabled", "arkon-boot.service")`.
- [ ] Format "● Workload Startup Pipeline" section with provider, profile order, and boot unit state in `internal/status/format.go`.
- [ ] Format "● Lifecycle Hooks" section with active hook names and timeouts in `internal/status/format.go`.
- [ ] Add unit tests in `internal/status/status_test.go` and `internal/cli/runner_test.go` verifying text formatting and JSON envelope serialization.
