# 14: Catch-All Workload Teardown via Profile Wildcard

**What to build:** In Docker Compose v2, services tagged with profile annotations (such as `profiles: ["proxy"]`) are ignored by `docker compose stop` unless explicitly addressed via `--profile <name>` or `--profile "*"`. Update `internal/teardown/provider.go` so `StopResidual` executes `docker compose ... --profile "*" stop` instead of plain `docker compose ... stop`. This guarantees any untracked or residual containers across all declared profiles are stopped before host sleep.

**Blocked by:** none

**Status:** resolved

- [x] In `internal/teardown/provider.go`, update `DockerComposeProvider.StopResidual` to prepend `--profile "*"` before `stop`.
- [x] In `internal/teardown/provider_test.go`, update unit tests to assert `--profile "*"` is passed to Docker Compose during residual stopping.
- [x] In `internal/teardown/coordinator_test.go`, verify mock and pipeline tests pass with the updated residual stop arguments.
