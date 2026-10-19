# 20: Documentation & Manifest Alignment

**What to build:** Align `README.md` and repository documentation with the complete v0.1.0 feature set, including updated configuration examples, accurate status previews, quickstart installer instructions, and reference manifest.

**Blocked by:** 16: Status Command Startup & Hooks Visibility, 18: GoReleaser Packaging & Release Automation, 19: Workload Provider Extensibility Documentation & Roadmap

**Status:** resolved

- [x] Create `arkon.example.yaml` in the root repository containing every configuration key with comprehensive comments.
- [x] Update `README.md` with the full `arkon.yaml` example (including `startup:` and `hooks:`).
- [x] Update `README.md` with the live borderless left-rail status preview.
- [x] Add quickstart installation instructions (curl one-liner and `.deb` package install) to `README.md`.
- [x] Add roadmap section linking to provider contribution issues.
