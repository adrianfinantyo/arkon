# ADR 0002: Binary Distribution and Self-Update Architecture

- **Status:** accepted
- **Date:** 2026-10-19
- **Deciders:** Adrian Finantyo

## Context

Arkon targets headless homelab servers running Linux, primarily Ubuntu and Debian distributions on AMD64 and ARM64 architectures. Operators need simple, reliable ways to install and update Arkon without maintaining a local Go build environment.

The distribution strategy must satisfy several constraints:

1. **Zero runtime dependencies:** Arkon is built as a single static binary (`CGO_ENABLED=0`) with no external library requirements.
2. **Standard package manager support:** Ubuntu and Debian homelabs benefit from `.deb` packages managed by `dpkg` or `apt`, allowing standard package inspection and clean removal.
3. **Frictionless one-liner installation:** Automated provisioning scripts and first-time users need a standalone shell installer (`scripts/install.sh`) that verifies checksums and handles architecture detection.
4. **Future in-place self-update (`arkon update`):** Operators managing machines through SSH or automated cron jobs should eventually be able to run `arkon update` to check for newer releases and upgrade the running binary in place.

## Decision

We adopt a two-tier distribution strategy combining automated GoReleaser packaging on git tags with an architecture plan for a future `arkon update` CLI command.

### 1. Release Packaging Pipeline (GoReleaser)

We use GoReleaser v2 triggered by GitHub Actions on tags matching `v*`:

- **Binary builds:** Cross-compile static Linux binaries for `amd64` and `arm64` with stripped debug info (`-s -w`) and link-time version stamping (`main.version`, `main.commit`, `main.date`).
- **Archive packages:** Bundle the compiled binary, `LICENSE`, and `README.md` into tarballs formatted as `arkon_{version}_linux_{arch}.tar.gz`.
- **Debian packages:** Generate `.deb` packages via NFPM matching `arkon_{version}_linux_{arch}.deb`, installing the binary to `/usr/local/bin/arkon`.
- **Integrity verification:** Compute SHA-256 checksums published in `checksums.txt` alongside release assets on GitHub Releases.

### 2. Standalone Install Script

An installation script (`scripts/install.sh`) is provided for automated setups:

- Detects host architecture (`x86_64` to `amd64`, `aarch64`/`arm64` to `arm64`) and confirms the OS is Linux.
- Resolves the requested or latest release tag from the GitHub Releases API.
- Downloads the corresponding archive and `checksums.txt`.
- Verifies archive integrity using `sha256sum`.
- Extracts and moves the binary to `/usr/local/bin/arkon` (prompting for `sudo` if the directory is not directly writable).

### 3. Future Self-Update Architecture (`arkon update`)

Scheduled for development in v0.2.0, the `arkon update` command will follow these design specifications:

- **Release Discovery:** Query `https://api.github.com/repos/adrianfinantyo/arkon/releases/latest` with standard User-Agent headers. Support an optional `--channel` flag (`stable` vs `prerelease`) and `--check-only` flag to report available upgrades without installing.
- **Version Comparison:** Parse semantic versions using semantic version comparison. If the current binary version matches or is newer than the remote release, exit cleanly with zero code.
- **Download and Verification:** Download the target architecture tarball and `checksums.txt` into a secure temporary directory (`/tmp/arkon-update.XXXXXX` with mode `0700`). Verify the archive SHA-256 checksum against `checksums.txt` before extraction.
- **Atomic File Replacement:**
  1. Extract the new binary as `/usr/local/bin/arkon.new`.
  2. Grant executable permissions (`0755`).
  3. Validate the replacement binary by executing `/usr/local/bin/arkon.new version` in a sandboxed subprocess.
  4. Perform an atomic rename syscall (`os.Rename("/usr/local/bin/arkon.new", "/usr/local/bin/arkon")`), which replaces the existing file inode cleanly even if currently running in another process on Linux.
  5. If the atomic rename fails or validation fails, delete the temporary file and abort without touching the active binary.
- **Privilege Separation:** If `/usr/local/bin` is not writable by the executing user, fail early with a clear message instructing the user to run `sudo arkon update`.

## Consequences

### Positive

- Homelab operators get pre-built binaries and native Debian packages for both x86-64 and ARM64 appliances.
- Releases are fully automated and reproducible from tagged Git commits.
- Version, commit hash, and build timestamp are stamped into every binary at build time.
- The path to a self-updating binary is documented and adheres to atomic replacement practices on Linux.

### Negative

- Self-updating via `arkon update` can bypass distribution package managers if a user installed via `.deb`. To mitigate this, `arkon update` will check whether `/usr/local/bin/arkon` is managed by `dpkg` (via `dpkg -S /usr/local/bin/arkon`) and recommend `apt upgrade` instead when package management is detected.
- Relying on the GitHub Releases API subject to rate limiting for unauthenticated requests (60 requests per hour per IP). `arkon update` will respect `GITHUB_TOKEN` environment variables when present.
