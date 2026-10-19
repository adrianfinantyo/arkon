# Pluggable Workload Providers

Arkon manages host power lifecycles for low-power Linux servers and homelabs. A core architectural principle is separating physical host power transitions from workload runtimes. Workload orchestration is handled through pluggable Go interfaces: `TeardownProvider` and `StartupProvider`.

This document explains the provider architecture, interface contracts, and how to implement a new workload engine.

---

## Architecture and Philosophy

### 1. CLI-First and Zero Resident Memory

Arkon is not a long-running daemon. It executes ephemerally via systemd timers, command-line invocations, and systemd sleep hooks.

* **0 MB Idle RAM:** When your server is running normally or sleeping, Arkon consumes no memory.
* **Low Peak Footprint:** During active execution (`teardown`, `up`, `apply`), Arkon stays well below 15 MB heap usage.
* **Pluggable Engine:** Physical power logic (RTC programming, ACPI sleep, kernel buffer sync) remains decoupled from workload runtimes (Docker, Podman, Kubernetes, systemd slices).

```
+---------------------------------------------------------------+
|                       Host Lifecycle                          |
|  (arkon-sleep.timer -> arkon sleep -> ACPI S3/S5 -> RTC wake) |
+-------------------------------+-------------------------------+
                                |
                 +--------------+--------------+
                 |                             |
                 v                             v
       Teardown Pipeline                Startup Pipeline
      (internal/teardown)              (internal/startup)
                 |                             |
                 v                             v
        TeardownProvider                StartupProvider
           [Interface]                     [Interface]
                 |                             |
      +----------+----------+       +----------+----------+
      |          |          |       |          |          |
      v          v          v       v          v          v
   Docker      Podman      K8s    Docker      Podman      K8s
   Compose    Compose    (Drain)  Compose    Compose   (Uncordon)
  (Built-in)  (Issue)    (Issue) (Built-in)  (Issue)    (Issue)
```

---

## Interface Definitions

### TeardownProvider

Defined in `internal/teardown/provider.go`:

```go
type TeardownProvider interface {
    Name() string
    CheckLiveness(ctx context.Context) error
    StopTier(ctx context.Context, tier string, timeout time.Duration) error
    StopResidual(ctx context.Context, timeout time.Duration) error
}
```

#### Method Contracts

1. **`Name() string`**
   * Returns the canonical provider identifier (e.g., `"docker-compose"`, `"podman-compose"`, `"kubernetes"`, `"systemd"`).
   * Matches the `teardown.provider` string in `arkon.yaml`.

2. **`CheckLiveness(ctx context.Context) error`**
   * Verifies that the required CLI binary or socket connection is operational before teardown begins.
   * If the binary is missing or the daemon is unreachable, returns an actionable error string including CLI output.

3. **`StopTier(ctx context.Context, tier string, timeout time.Duration) error`**
   * Gracefully stops all containers or workloads matching the specified profile tier (for example, `"workers"`, `"web"`, or `"storage"`).
   * Runs sequentially according to `teardown.profile_order` in the manifest.
   * Must pass the per-tier timeout to the underlying engine stop command.
   * If any container fails to stop within the timeout, returns an error to trigger Arkon's fail-safe abort.

4. **`StopResidual(ctx context.Context, timeout time.Duration) error`**
   * Acts as a catch-all sweep after configured profile tiers have finished stopping.
   * Stops unprofiled, untagged, or remaining workloads across registered stacks.
   * Ensures no running containers write dirty pages to disk while Arkon runs kernel cache sync.

---

### StartupProvider

Defined in `internal/startup/provider.go`:

```go
type StartupProvider interface {
    Name() string
    CheckLiveness(ctx context.Context) error
    StartStack(ctx context.Context, stack string, profiles []string, timeout time.Duration) error
}
```

#### Method Contracts

1. **`Name() string`**
   * Returns the canonical provider identifier.
   * Matches the `startup.provider` string in `arkon.yaml`.

2. **`CheckLiveness(ctx context.Context) error`**
   * Verifies the container engine or orchestrator is available post-boot or post-wake before issuing start commands.

3. **`StartStack(ctx context.Context, stack string, profiles []string, timeout time.Duration) error`**
   * Brings up workloads for a target stack path or descriptor.
   * When `profiles` contains entries, iterates through profiles in order and brings up matching services.
   * When `profiles` is empty, starts all services in the target stack.
   * Must respect the specified context and timeout.

---

## Step-by-Step Guide: Contributing a New Provider

Follow these steps to contribute a new workload provider (e.g., Podman Compose, Kubernetes node drain, or systemd target slices).

### Step 1: Implement the Interfaces

Create your provider implementation in the relevant packages or a dedicated subpackage:

* Teardown logic in `internal/teardown/podman.go` (or `internal/teardown/<provider>.go`).
* Startup logic in `internal/startup/podman.go` (or `internal/startup/<provider>.go`).

**Rule: Use `env.HostEnvironment` for all host interactions.**
Never call `exec.Command`, `os.ReadFile`, or `os.Getenv` directly. Accept `env.HostEnvironment` in your provider constructor so operations can be verified in tests without root privileges or host binaries.

Example skeleton for teardown:

```go
package teardown

import (
    "context"
    "fmt"
    "time"

    "github.com/adrianfinantyo/arkon/internal/env"
)

type PodmanComposeProvider struct {
    env    env.HostEnvironment
    stacks []string
}

func NewPodmanComposeProvider(environment env.HostEnvironment, stacks ...string) *PodmanComposeProvider {
    return &PodmanComposeProvider{
        env:    environment,
        stacks: stacks,
    }
}

func (p *PodmanComposeProvider) Name() string {
    return "podman-compose"
}

func (p *PodmanComposeProvider) CheckLiveness(ctx context.Context) error {
    out, err := p.env.Exec(ctx, "podman", "compose", "version")
    if err != nil {
        return fmt.Errorf("podman compose is not available: %w (output: %s)", err, string(out))
    }
    return nil
}

func (p *PodmanComposeProvider) StopTier(ctx context.Context, tier string, timeout time.Duration) error {
    // Execute podman compose stop for the specified tier...
    return nil
}

func (p *PodmanComposeProvider) StopResidual(ctx context.Context, timeout time.Duration) error {
    // Execute podman compose stop for residual containers...
    return nil
}
```

---

### Step 2: Register in Teardown and Startup Factories

Connect the provider to the coordinator constructors:

1. In `internal/teardown/coordinator.go`:
   Update `NewCoordinator` (or create a factory function) to instantiate your provider when `m.Teardown.Provider` matches your provider key.

2. In `internal/startup/coordinator.go`:
   Update `NewCoordinator` to instantiate your provider when `m.Startup.Provider` matches your provider key.

---

### Step 3: Update Manifest Validation

Add the provider key to the accepted list in `internal/manifest/validation.go`:

```go
var validProviders = map[string]bool{
    "docker-compose": true,
    "podman-compose": true,
    "kubernetes":     true,
    "command":        true,
    "systemd":        true,
}
```

Verify that validation errors are returned when an unsupported provider string is configured in `arkon.yaml`.

---

### Step 4: Write Tests with `env.MemEnv`

All provider unit tests run against `env.MemEnv`, an in-memory simulation of the host environment. This allows checking command invocations, standard error handling, and timeout behavior without needing real daemons.

Example test pattern:

```go
func TestPodmanComposeProvider_StopTier(t *testing.T) {
    mem := env.NewMemEnv()
    mem.RegisterExecHandler(func(ctx context.Context, name string, args ...string) ([]byte, error) {
        if name == "podman" && args[0] == "compose" {
            return []byte("stopped"), nil
        }
        return nil, fmt.Errorf("unexpected command: %s", name)
    })

    provider := NewPodmanComposeProvider(mem, "/opt/apps")
    err := provider.StopTier(context.Background(), "web", 10*time.Second)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
}
```

---

## Provider Roadmap

Tracked roadmap issues for additional providers:

1. **Rootless Podman Compose Provider:** Support systemd user sessions and `podman compose` subcommands.
2. **Kubernetes / K3s Node Drain Provider:** Single-node homelab clusters using `kubectl drain` before sleep and `kubectl uncordon` after wake.
3. **Systemd Target Slice Provider:** Managing bare-metal services grouped into systemd slices (`systemctl stop <slice>`).

Contributors interested in implementing these providers are encouraged to claim the corresponding GitHub issues.
