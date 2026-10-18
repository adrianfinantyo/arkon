# [04] Inhibition Locks & State Management

Type: grilling
Status: resolved
Blocked by: 01

## Question

How should `arkon inhibit` record temporary shutdown cancellations, daytime locks, and maintenance windows on the Root SSD (`/var/lib/arkon/state.json`) so that the automated nightly shutdown timer respects active locks, auto-expires stale inhibitions, and provides clear diagnostic reasons via `arkon status`?

## Answer

Ticket resolved through design tree grilling.

### 1. State File Location & Atomic Locking

* **Location:** Runtime state is stored at `/var/lib/arkon/state.json` on the Root SSD.
* **Concurrency Protection:** Guarded by an advisory file lock (`syscall.Flock`) on `/var/lib/arkon/state.lock`.
* **Power-Safe Atomic Writes:** State writes serialize to a temporary file (`os.CreateTemp` in `/var/lib/arkon/`) and are atomically swapped into place via `os.Rename`, guaranteeing that sudden power interruptions never leave a corrupted JSON file.

### 2. Inhibition Lock Schema & Auto-Expiration

* **Schema:**
  ```json
  {
    "id": "inh-20261018-01",
    "reason": "Database migration in progress",
    "created_at": "2026-10-18T22:30:00Z",
    "expires_at": "2026-10-19T00:30:00Z",
    "created_by": "adrian"
  }
  ```
* **Self-Cleaning Lifecycle:** Every lock has an explicit `expires_at` timestamp calculated from `--duration` (defaulting to `manifest.inhibition.default_duration`).
* **Stale Pruning:** Whenever `arkon sleep` or `arkon status` runs, expired locks (`now >= expires_at`) are automatically pruned and persisted, ensuring forgotten maintenance overrides do not disable host power cycling indefinitely.

### 3. Daytime Operational Protection

* When `manifest.inhibition.daytime_protection: true`, `arkon sleep` checks whether the current local time falls between `circadian.wake_at` and `circadian.sleep_at`.
* **Violation Messaging:** If invoked during daytime hours, `arkon sleep` aborts and returns an informative error message:
  `"Refusing sleep: current time HH:MM is within daytime operational window (wake_at: HH:MM, sleep_at: HH:MM). Pass --force to override."`
* Returns exit code `2` (Inhibited).

### 4. CLI Ergonomics (`arkon inhibit`)

* `arkon inhibit`: Creates a lock using the default duration from manifest (e.g. `2h`).
* `arkon inhibit -d 4h -r "Backup in progress"`: Sets custom duration and descriptive reason.
* `arkon inhibit --release [id]` or `arkon inhibit --clear`: Cancels active locks immediately.
* `arkon inhibit --list`: Displays active locks, creators, and remaining time.

### 5. Machine Contract & AI Agent Integration

* When `arkon sleep` is blocked by an active lock or daytime protection:
  - Exit code: `2` (Inhibited).
  - Terminal output: Detailed error with lock ID, reason, remaining time, and command to clear (`arkon inhibit --release`).
  - Structured JSON under `--json`:
    ```json
    {
      "success": false,
      "inhibited": true,
      "code": "ERR_INHIBITED",
      "message": "Sleep transition inhibited by active lock",
      "active_lock": {
        "id": "inh-20261018-01",
        "reason": "Database migration in progress",
        "expires_at": "2026-10-19T00:30:00Z",
        "remaining": "1h 14m"
      }
    }
    ```
  - Emits an audit log entry to journald for host observability.
