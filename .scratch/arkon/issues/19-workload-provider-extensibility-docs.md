# 19: Workload Provider Extensibility Documentation & Roadmap

**What to build:** Document the pluggable workload provider interfaces (`TeardownProvider` and `StartupProvider`) for community contributors and track Podman, Kubernetes, and systemd slices on GitHub issues.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Create `docs/architecture/workload-providers.md` explaining the provider architecture, method contracts, and step-by-step contribution guide.
- [x] Create GitHub issue for Podman Compose workload provider with `enhancement` and `help wanted` labels (#21).
- [x] Create GitHub issue for Single-Node Kubernetes / K3s drain provider with `enhancement` and `help wanted` labels (#22).
- [x] Create GitHub issue for Systemd Target Slice workload provider with `enhancement` and `help wanted` labels (#23).
- [x] Reference the provider documentation in `CONTRIBUTING.md`.
