# [05] CLI Command Surface & Terminal UX

Type: prototype
Status: resolved
Blocked by: 01, 04

## Question

How should the terminal user experience be structured for Arkon's four primary commands (`status`, `apply`, `sleep`, `inhibit`), how should system diagnostics be formatted for human operators in tables vs machine-readable JSON (`--json`), and how should Linux permission checks / `sudo` elevation be enforced gracefully?

## Answer

Ticket resolved through interactive prototyping and terminal safety evaluation.

* **Prototype Assets:**
  - Python Runner: [`.scratch/arkon/prototype/arkon-prototype.py`](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/prototype/arkon-prototype.py)
  - Go Implementation: [`.scratch/arkon/prototype/main.go`](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/prototype/main.go)

### 1. Single Unified Left-Rail Layout (Zero Multi-View Overhead)

Rather than maintaining dual rendering engines or fragile horizontal box borders (`╭───╮`, `╰───╯`) that crack and line-wrap on narrow screens, Arkon adopts a **single, unified, borderless left-rail layout** (modeled after standard Unix tools like `systemctl status` and `git status`):

* **Left-Rail Bullets:** Uses colored bullets (`●`) and clear two-space indented key-value pairs.
* **Immune to Window Dimensions:** Naturally flows vertically without hardcoded horizontal widths, looking clean on 50-column mobile terminals, 80-column standard SSH, or 200-column desktop windows.
* **Zero Bloat:** Single straightforward printer function; zero dynamic column sniffing, zero duplicate rendering paths.
* **NO_COLOR & Pipe Compliant:** Strips ANSI escape codes automatically when `NO_COLOR` is set or stdout is piped (`!isatty(os.Stdout)`).

### 2. Machine Contract & AI Agent Integration (`--json`)

* Every command supports `--json`, outputting a deterministic envelope:
  ```json
  {
    "success": true,
    "data": { ... },
    "error": null
  }
  ```
* Standardized error payload with machine-readable error codes (`ERR_INHIBITED`, `ERR_DAYTIME_PROTECTION`, `ERR_INSUFFICIENT_PERMISSIONS`).
* Completely independent of terminal dimensions, ANSI escape codes, or TTY states.

### 3. Deterministic Exit Codes

* `0`: Success / OK
* `1`: General Runtime Error
* `2`: Power Transition Inhibited (Active Lock or Daytime Protection)
* `3`: Manifest Validation / Syntax Failure
* `4`: Missing Root Privileges (with clear guidance to use `sudo arkon apply` or `arkon apply --dry-run`)

### 4. Actionable Diagnostic Error Cards

* Operational conflicts render formatted diagnostic cards explaining:
  - Exact violation reason (e.g. Daytime window 07:00 - 23:00 vs current time 14:30).
  - Active lock metadata (Lock ID, reason, expiration time).
  - Concrete resolution commands (`arkon inhibit --release` or `arkon sleep --force`).
