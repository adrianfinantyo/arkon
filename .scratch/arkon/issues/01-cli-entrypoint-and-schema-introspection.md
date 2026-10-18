# 01: CLI Entrypoint, Environment Seam & Schema Introspection

**What to build:** The command-line routing foundation for Arkon with global `--json` envelope formatting, deterministic exit codes (0, 1, 2, 3, 4), and the `HostEnvironment` testing seam. Operators and automated tools can inspect canonical JSON Schemas for manifests and runtime state via `arkon schema`, and receive structured error envelopes whenever commands fail or receive invalid arguments.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] Passing `-h` or `--help` prints the top-level command usage listing all subcommands (`apply`, `status`, `sleep`, `inhibit`, `schema`, `version`).
- [ ] Running `arkon version` outputs binary version, git commit, and build date.
- [ ] Running `arkon schema` prints the complete Draft 2020-12 JSON Schema for manifests and state files to standard output.
- [ ] Passing the global `--json` flag wraps all command outputs in a typed JSON envelope containing `success` (boolean), `data` (object or null), and `error` (code, message, details or null).
- [ ] The CLI enforces standard exit codes: `0` (Success), `1` (General Error), `2` (Inhibited), `3` (Validation/Compilation Failure), `4` (Insufficient Privileges).
- [ ] Unknown subcommands return exit code 1 with an actionable diagnostic message.
- [ ] The `HostEnvironment` interface and command runner are established so all subsequent CLI commands can be exercised end-to-end in automated tests without root privileges.
