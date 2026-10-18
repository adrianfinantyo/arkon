# 05: Hardware RTC Wakealarm Driver & ACPI Subsystem Abstraction

**What to build:** Low-level Linux Real-Time Clock sysfs driver and ACPI power action execution. Operators can program hardware wakealarms directly via `/sys/class/rtc/rtc0/wakealarm` using relative seconds without runtime subprocess dependencies on `rtcwake`, inspect wake timestamps mapped from `/etc/adjtime`, and trigger system power actions safely.

**Blocked by:** 01: CLI Entrypoint, Environment Seam & Schema Introspection

**Status:** ready-for-agent

- [ ] Clears existing hardware alarms by writing `0\n` to `/sys/class/rtc/rtc0/wakealarm` before setting a new alarm, preventing kernel `-EBUSY` failures.
- [ ] Writes relative seconds (`+<seconds>\n`) to `/sys/class/rtc/rtc0/wakealarm`, ensuring write operations remain invariant to BIOS UTC vs local clock configuration.
- [ ] Reads back `/sys/class/rtc/rtc0/wakealarm` immediately following a write to verify hardware retention before system power drops.
- [ ] Parses `/etc/adjtime` line 3 (`UTC` or `LOCAL`) to accurately translate sysfs readback integer epochs to local host wall-clock time for display.
- [ ] Enforces a minimum safety buffer of 180 seconds, rejecting poweroff attempts if the wake target is inside the buffer window.
- [ ] Implements the ACPI host power transition driver, executing `poweroff`, `suspend`, or `hibernate` according to manifest configuration.
