# 04: Inhibition Locks, Daytime Protection & State Engine (`arkon inhibit`)

**What to build:** The runtime state persistence layer at `/var/lib/arkon/state.json` guarded by atomic advisory file locking (`syscall.Flock`). Operators and external scripts can pause automated scheduled shutdowns using `arkon inhibit`, release active locks via `arkon inhibit release <id>`, list active overrides, and automatically purge expired locks without human intervention.

**Blocked by:** 01: CLI Entrypoint, Environment Seam & Schema Introspection

**Status:** done

- [x] Persists runtime state to `/var/lib/arkon/state.json` using atomic temporary file write and `os.Rename`.
- [x] Concurrency across CLI commands is guarded by `syscall.Flock` on `/var/lib/arkon/state.lock`.
- [x] Running `arkon inhibit --reason "<text>" --duration "<time>"` creates an active inhibition lock recording ID, reason, expiration time, and invoking username.
- [x] Running `arkon inhibit list` outputs all currently active inhibition locks with remaining duration.
- [x] Running `arkon inhibit release <id>` deletes the specified lock from state, or returns an error if the ID is not found.
- [x] Expired locks (`now >= expires_at`) are automatically pruned from state upon read and persisted back to disk.
- [x] Evaluates daytime operational protection window when enabled, signaling that daytime sleep transitions are inhibited unless overridden with `--force`.
- [x] Machine contract returns exit code 2 when any active lock or daytime protection condition blocks a sleep transition.
