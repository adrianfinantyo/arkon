# Arkon Specification: Release Surround & v0.1.0 Readiness

Status: ready-for-agent

## Problem Statement

Arkon's core execution engine (manifest compilation, systemd units, container profile teardown, boot startup, RTC wakealarms, and inhibition locks) is functional and verified by tests. However, the project lacks the operational "surround" needed to safely ship v0.1.0 to homelab operators and establish an open foundation for outside contributors:

1. **Status Visibility Gap:** Running `arkon status` on a deployed host (such as pilot server `srv-komsos`) hides the startup pipeline and lifecycle hooks. The operator cannot verify boot profile order or hooks without opening raw YAML files.
2. **Distribution & Version Stamping Absence:** Binaries built manually display `version: dev (commit: none)`. There is no automated release pipeline, checksum verification, or Debian package generation for Ubuntu/Debian homelab appliances.
3. **Local Lint & CI Parity Friction:** Developers and agents occasionally hit CI failures due to unformatted files (`go fmt`) or lint violations that passed locally because git hooks only ran basic checks or were skipped before pushing.
4. **Unclear Extensibility for Non-Docker Workloads:** Arkon's mission includes Podman, Kubernetes, and systemd slices, but community contributors lack clear documentation of the `TeardownProvider` and `StartupProvider` Go interfaces and have no tracked contribution tickets to claim.
5. **Documentation Drift:** `README.md` reflects an early design without `startup:` or accurate terminal status previews.

## Solution

Build out the complete v0.1.0 release surround:

1. **Status Reporter Completeness:** Extend `internal/status` to collect and format `StartupStatus` and `HooksStatus` across both the borderless terminal view and the typed `--json` envelope.
2. **Local CI Parity & Pre-Push Hook:** Introduce a canonical `./scripts/check-ci.sh` running exact CI steps (`go mod verify`, `test -z "$(go fmt ./...)"`, `go vet ./...`, `python3 scripts/deslop.py --all --force`, `go test -v ./...`), wire `.githooks/pre-push` to run it, and keep `.githooks/pre-commit` fast and focused.
3. **GoReleaser Automation & Version Stamping:** Configure `.goreleaser.yaml` to produce static `linux/amd64` and `linux/arm64` archives, `.deb` packages, and SHA256 checksums on git tags. Provide an official `scripts/install.sh` for one-liner curl installation, and record an ADR for future self-update (`arkon update`).
4. **Extensibility Architecture & Community Roadmap:** Document workload provider seams in `docs/architecture/workload-providers.md`, and open GitHub contribution issues for Podman and Kubernetes providers.
5. **Documentation & Manifest Alignment:** Update `README.md` with current syntax, add a fully annotated `arkon.example.yaml` reference manifest, and prepare the v0.1.0 release.

## User Stories

1. As a homelab operator running `arkon status`, I want to see the configured workload startup pipeline, boot systemd unit state, and boot profile progression, so that I can verify morning wake behavior at a glance.
2. As a homelab operator running `arkon status`, I want to inspect all registered lifecycle hooks (pre-teardown, post-teardown, pre-sleep, post-wake), so that I know exactly which host maintenance scripts will execute.
3. As a monitoring script or home automation tool running `arkon status --json`, I want structured `startup` and `hooks` objects in the JSON payload, so that I can ingest boot and hook status programmatically.
4. As a contributor or maintainer, I want git pre-push hooks and a local `./scripts/check-ci.sh` script to run the exact CI test and lint suite before pushing, so that broken commits never fail GitHub Actions CI.
5. As an open-source contributor, I want git pre-commit checks to remain fast (under 1 second) while catching formatting and anti-slop issues, so that local commit flow remains smooth.
6. As a homelab operator installing Arkon on Ubuntu or Debian, I want pre-compiled `.deb` packages and `.tar.gz` release assets on GitHub, so that I can install Arkon without setting up a Go build toolchain.
7. As a homelab operator, I want to run `curl -fsSL https://raw.githubusercontent.com/adrianfinantyo/arkon/main/scripts/install.sh | sudo bash`, so that Arkon installs directly to `/usr/local/bin/arkon` with automatic architecture detection.
8. As a user checking `arkon version`, I want to see the tagged release version, git commit hash, and build date, so that I can verify which build is active on my host.
9. As a future maintainer, I want an architectural design record (ADR) for self-updating (`arkon update`), so that automatic in-place binary upgrades can be implemented cleanly in a later release.
10. As a Podman or Kubernetes user, I want clear documentation explaining how to implement `TeardownProvider` and `StartupProvider`, so that I can contribute support for non-Docker container engines.
11. As a community contributor, I want GitHub issues tracking Podman and Kubernetes provider implementations, so that I can claim and implement them.
12. As a new user setting up Arkon for the first time, I want an annotated `arkon.example.yaml` showing every available configuration parameter, so that I do not need to guess valid values or syntax.
13. As an operator reading `README.md`, I want the quickstart and status outputs to match the real software behavior, so that instructions are accurate.

## Implementation Decisions

### 1. Status Model Expansion
- In `internal/status/types.go`:
  - Add `Startup StartupStatus` and `Hooks HooksStatus` fields to `StatusData`.
  - Define `StartupStatus` with `Provider`, `Profiles`, `StackCount`, `Stacks`, and `BootUnit` (active/enabled/inactive).
  - Define `HooksStatus` with `PreTeardown`, `PostTeardown`, `PreSleep`, and `PostWake` string slices.
- In `internal/status/collector.go`:
  - Implement `collectStartup(ctx, hostEnv, m)` and `collectHooks(m)`.
  - Check `arkon-boot.service` unit state via `hostEnv.Exec(ctx, "systemctl", "is-enabled", "arkon-boot.service")` when real or simulated.
- In `internal/status/format.go`:
  - Render "● Workload Startup Pipeline" section with provider, profile progression, and boot unit state.
  - Render "● Lifecycle Hooks" section listing active hook names and timeouts if any are configured.

### 2. Git Hooks & CI Parity
- Add `scripts/check-ci.sh` as the single source of truth for repository health:
  - `go mod verify`
  - `test -z "$(go fmt ./...)"`
  - `go vet ./...`
  - `python3 scripts/deslop.py --all --force`
  - `go test -v ./...`
- Update `.githooks/pre-commit`:
  - Run fast checks only: check staged files or run `go fmt` diff check and `deslop.py`.
- Add `.githooks/pre-push`:
  - Execute `./scripts/check-ci.sh`. Reject the push if any check fails.
- Document in `CONTRIBUTING.md` how contributors activate hooks (`git config core.hooksPath .githooks`).

### 3. GoReleaser and Distribution
- Create `.goreleaser.yaml`:
  - Target `linux/amd64` and `linux/arm64`.
  - Static compilation (`CGO_ENABLED=0`).
  - Ldflags: `-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}`.
  - Archive format: `.tar.gz` containing binary, `LICENSE`, `README.md`, `arkon.example.yaml`.
  - Package formats: `.deb` package generation via NFPM.
  - Checksums: SHA256 checksums file.
- Create `.github/workflows/release.yml`:
  - Triggered on tag push (`v*`).
  - Runs GoReleaser with GitHub token to create draft or published GitHub Release with attached assets.
- Create `scripts/install.sh`:
  - Detects `uname -s` (must be Linux) and `uname -m` (`x86_64` -> `amd64`, `aarch64`/`arm64` -> `arm64`).
  - Fetches latest release metadata from GitHub API.
  - Downloads `.tar.gz`, verifies checksum, and installs binary to `/usr/local/bin/arkon`.

### 4. Workload Provider Seam Documentation
- Create `docs/architecture/workload-providers.md` documenting:
  - The design philosophy: CLI-first orchestration, zero persistent daemon.
  - The `teardown.TeardownProvider` interface and method contracts.
  - The `startup.StartupProvider` interface and method contracts.
  - Step-by-step instructions for adding a new provider (e.g., `podman` or `k8s`).
- Open GitHub issues with labels `enhancement` and `help wanted`:
  - "Feature: Add Podman Compose Workload Provider"
  - "Feature: Add Single-Node Kubernetes / K3s Drain Provider"
  - "Feature: Add Systemd Target Slice Workload Provider"

### 5. Reference Manifest & Documentation
- Create `arkon.example.yaml` in root with comprehensive comments.
- Update `README.md` to show the full manifest, accurate terminal status output, install instructions, and provider roadmap.

## Testing Decisions

- Test `internal/status/collector_test.go` and `format_test.go` for `startup` and `hooks` rendering, ensuring 100% test coverage.
- Verify `arkon status --json` output contains valid `startup` and `hooks` properties.
- Test `scripts/check-ci.sh` execution.
- Test `goreleaser check` to validate GoReleaser configuration syntax.
- Test cross-compilation with ldflags build metadata stamping.

## Out of Scope

- Implementing the actual Podman or K8s provider engines (tracked as separate community issues).
- Building an in-binary auto-update CLI command (`arkon update`) in v0.1.0 (architectural design documented in ADR for v0.2.0).
- Windows or macOS support (Arkon targets Linux ACPI and systemd).
