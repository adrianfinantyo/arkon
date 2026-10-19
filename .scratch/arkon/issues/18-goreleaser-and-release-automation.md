# 18: GoReleaser Packaging & Release Automation

**What to build:** Configure GoReleaser and GitHub Actions release workflow for automated cross-compiled binary distribution (`linux/amd64`, `linux/arm64`), `.deb` packaging, version metadata stamping, and curl installer.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Create `.goreleaser.yaml` generating static binaries (`CGO_ENABLED=0`), `.tar.gz` archives, `.deb` packages via nfpm, and SHA256 checksums.
- [x] Wire build ldflags in `.goreleaser.yaml` to stamp `main.version`, `main.commit`, and `main.date`.
- [x] Create `.github/workflows/release.yml` triggered on git tags (`v*`) to build and publish release assets to GitHub Releases.
- [x] Create `scripts/install.sh` for one-liner curl installation with architecture detection and checksum verification.
- [x] Record ADR `docs/adr/0002-binary-distribution-and-self-update.md` outlining the architecture for future `arkon update`.
