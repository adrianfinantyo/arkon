# 15: Declarative Startup Orchestrator & Boot Systemd Unit

**What to build:** Extend Arkon to manage host startup and morning container boot, unifying full boot-to-shutdown operations into `arkon.yaml` and retiring ad-hoc boot systemd units.
1. Add a declarative `startup:` configuration section to `arkon.yaml` parameterizing startup provider, stacks, timeout, and profile list.
2. Implement the `arkon up` CLI command to execute post-wake hooks (such as storage drive detachment or preparation) and start container stacks for configured profiles.
3. Update `internal/systemd` to compile and install an `arkon-boot.service` unit running on `multi-user.target` after `docker.service` and `network-online.target`.

**Blocked by:** 14: Catch-All Workload Teardown via Profile Wildcard

**Status:** resolved

- [x] Add `StartupConfig` to `internal/manifest/types.go` and wire into `Manifest` struct.
- [x] Add default application and validation rules for `StartupConfig` in `internal/manifest/defaults.go` and `validation.go`.
- [x] Update canonical JSON schema in `internal/cli/schema.go`.
- [x] Create `internal/systemd/templates/arkon-boot.service.tmpl`.
- [x] Update `internal/systemd/compiler.go` and `installer.go` to compile, verify, and atomically install `arkon-boot.service`.
- [x] Implement `arkon up` command in `internal/cli/runner_up.go` and wire into `Runner.Run`.
- [x] Add unit tests for manifest parsing, startup systemd unit compilation/installation, and `arkon up` CLI execution.
