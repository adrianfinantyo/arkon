# 21: Active Host Manifest Authority & Discovery Drift Notice

**What to build:** Establish single active host manifest authority through canonical symlink at `/etc/arkon/arkon.yaml` and state record in `/var/lib/arkon/state.json`. Prevent directory confusion by having `arkon apply` register the active host authority, providing clear unconfigured/degradation warnings in `arkon status`, and detecting drift when local manifests differ from the host authority.

**Blocked by:** 15: Workload Startup Orchestration and Boot Unit, 16: Status Command Startup & Hooks Visibility

**Status:** resolved

- [x] Extend `env.HostEnvironment` seam with `Symlink` and `Readlink` methods.
- [x] Add `ActiveManifest`, `AppliedAt`, and `AppliedBy` to persistent host state schema in `/var/lib/arkon/state.json`.
- [x] Implement canonical symlink creation and state persistence in `systemd.Installer`.
- [x] Update manifest resolution: prioritize registered active host manifest, detect local directory drift, and surface warnings.
- [x] Render `[NOT CONFIGURED]`, `[BROKEN ACTIVE MANIFEST]`, and `● Active Manifest Notice` in `arkon status`.
- [x] Record ADR-0003 and update domain definitions in `GLOSSARY.md`.
