# 09: Host Environment Seam Integrity & CLI Runner Decomposition

**What to build:** Eliminate environment variable leaks across the `HostEnvironment` seam and decompose `internal/cli/runner.go` into focused, per-subcommand dispatchers. Ensure state directory resolution and invoking user identification strictly use the `HostEnvironment` abstraction instead of falling back to `os.Getenv` or `user.Current()`. Ensure `loadAndValidateManifest` returns an explicit diagnostic error code when a required manifest is not found instead of returning `(nil, "", ExitOK)`.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `resolveStateDir` in CLI runner strictly queries `r.Env.LookupEnv("ARKON_STATE_DIR")` without bypassing `HostEnvironment` to `os.Getenv`.
- [x] `invokingUser` in CLI runner queries `r.Env.LookupEnv("SUDO_USER")` and `r.Env.LookupEnv("USER")` before falling back to default "operator", eliminating unmocked `user.Current()` and `os.Getenv` leaks.
- [x] `loadAndValidateManifest` requires an explicit boolean or strict validation mode for commands that mandate a manifest (`apply`, `status`, `sleep`, `wake`), returning exit code 3 when discovery fails rather than returning nil manifest with exit code 0.
- [x] Eliminate duplicated privilege checking and error emission in `runApply`.
- [x] Decompose `runner.go` into modular dispatchers (`runner_apply.go`, `runner_sleep.go`, `runner_inhibit.go`, `runner_status.go`, `runner_wake.go`) to maintain long-term code clarity.
- [x] All existing and refactored CLI tests pass without regression.
