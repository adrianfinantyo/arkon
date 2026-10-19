# 08: Borderless Terminal UX & Status Reporter (`arkon status`)

**What to build:** The operator diagnostic dashboard and health reporter displaying host power state, hardware RTC wakealarms, circadian schedule countdowns, active inhibition locks, and teardown stack configuration in a clean vertical left-rail layout, with strict JSON output for monitoring scripts.

**Blocked by:** 02: Manifest Schema, Discovery Hierarchy & Validation Engine; 04: Inhibition Locks, Daytime Protection & State Engine (`arkon inhibit`); 05: Hardware RTC Wakealarm Driver & ACPI Subsystem Abstraction

**Status:** resolved

- [x] Formats status in a vertical borderless left-rail layout with colored bullets (`●`) and two-space indented key-value pairs, immune to line-wrapping on narrow screens.
- [x] Displays host metadata (hostname, CPU architecture, configured timezone).
- [x] Displays hardware RTC wakealarm state: enabled/disabled, scheduled local wall-clock time, and human-readable countdown remaining.
- [x] Displays circadian timer policy: sleep schedule, wake schedule, next trigger time, and systemd timer unit state.
- [x] Lists all currently active inhibition locks with reason, creator, and remaining validity time.
- [x] Displays teardown configuration summary: provider name, profile tiers, and registered compose stack counts.
- [x] Supports global `--json` emitting the complete typed `StatusData` envelope for programmatic monitoring without terminal escape codes.
- [x] Executes entirely without root privileges (unprivileged reads on `/sys/class/rtc/rtc0/wakealarm`, `/etc/adjtime`, and `/var/lib/arkon/state.json`).
