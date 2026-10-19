# [03] Profile-Aware Container Teardown Engine

Type: grilling
Status: resolved
Blocked by: 01

## Question

How should Arkon execute the phased teardown of Docker Compose container profiles (`apps` -> `infra` -> `*`) to guarantee clean database WAL checkpoints and buffer flushes, should it invoke the Docker CLI (`docker compose`) or speak directly to the Docker Engine Unix socket (`/var/run/docker.sock`), and how should per-stage timeouts be managed?

## Answer

Ticket resolved through 2 rounds of design tree grilling.

### 1. Pluggable Teardown Provider Seam (`TeardownProvider`)

To keep Arkon extensible across diverse homelab runtime environments (Docker, Podman, PM2, K3s), the engine is decoupled behind the `TeardownProvider` Go interface:

```go
type TeardownProvider interface {
    Name() string
    CheckLiveness(ctx context.Context) error
    StopTier(ctx context.Context, tier string, timeout time.Duration) error
    StopResidual(ctx context.Context, timeout time.Duration) error
}
```

* **Default Provider (`docker-compose`):** Invokes `docker compose --project-directory <path> --profile <tier> stop -t <sec>`, relying on native Compose profile, environment, and dependency resolution without embedding a bulky Compose YAML parser. Uses `/var/run/docker.sock` for fast pre-flight daemon health ping.
* **Extensible Providers:**
  - `podman-compose`: Identical CLI semantics for rootless/daemonless container hosts.
  - `command`: Arbitrary CLI hooks per tier (supporting PM2, custom scripts, or `kubectl drain`).
  - `systemd`: Stopping specific systemd target slices.

### 2. Phased Profile Progression & Residual Handling

* **Strict Tier Order:** Executes tiers sequentially according to `teardown.profile_order` (e.g., Tier 1: `apps`, Tier 2: `infra`).
* **Exit State Verification:** The engine polls and verifies that all containers belonging to a tier have transitioned to the `exited` state before initiating `SIGTERM` to the next tier.
* **Residual Catch-All:** Executes a final stop for any unprofiled or untagged containers in the stack to prevent orphaned processes from surviving into the sleep phase.

### 3. Kernel Page Cache & Database WAL Sync (`syscall.Sync()`)

* **Dirty Page Drain:** Clean container exit alone only guarantees database processes have flushed data into the Linux kernel OS page cache (volatile RAM).
* **Mandatory Sync:** Following container teardown, Arkon calls `syscall.Sync()` to force the kernel to flush dirty pages to physical disk storage (SSD NAND / HDD platters).
* **Buffer Monitoring:** Arkon monitors `/proc/meminfo` (`Dirty: <kb>`) to verify dirty pages drain to zero before hardware power transition begins.

### 4. Safety Invariant: Fail-Safe Abort

* **No Automated `SIGKILL`:** Arkon refuses to issue `SIGKILL` during automated scheduled sleep if a container exceeds its timeout.
* **Abort on Failure:** If a container hangs or fails to exit cleanly, Arkon aborts the circadian sleep cycle, leaves the host powered on, logs the offending container name to journald, and records the failure in state.
* **Operator Override:** Automated sleep always aborts on hang; only an operator passing an explicit `--force` flag can bypass this safeguard.

### 5. AI Agent & Observability Contract

* **Typed Lifecycle Events:** Emits granular events (`tier_started`, `container_stopped`, `tier_completed`, `sync_flushed`).
* **Dual Output:** Interactive progress spinner for human terminal sessions, and structured ndjson / streaming JSON under `--json` for autonomous AI agents.
