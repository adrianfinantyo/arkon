# 10: State File Corruption Resilience & Recovery

**What to build:** Resilient state file deserialization and self-healing in `internal/state`. When `/var/lib/arkon/state.json` is empty, truncated, or contains malformed JSON (e.g. following sudden power drop or manual tampering), Arkon must quarantine the damaged file, log an actionable diagnostic notice, and initialize a clean runtime state rather than permanently crashing and disabling host power lifecycle operations.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] `loadAndPruneState` in `internal/state/state.go` safely handles zero-byte files by returning an initialized state with an empty locks slice.
- [x] When `state.json` contains malformed or unparseable JSON, the manager backs up the corrupted file to `state.json.corrupt.<timestamp>` in the state directory and resets state cleanly.
- [x] Corrupted state recovery logs an actionable warning to stderr or records a diagnostic notice without crashing CLI invocations (`status`, `sleep`, `inhibit`).
- [x] Unit tests in `internal/state/state_test.go` exercise zero-byte files, truncated JSON chunks, and invalid syntax to verify automated quarantine and clean reset behavior.
