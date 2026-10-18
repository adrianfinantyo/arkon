# [02] Linux RTC Wake & ACPI S5 Abstraction

Type: research
Status: resolved
Blocked by: none

## Question

Should Arkon program the hardware wakealarm via direct sysfs writes (`/sys/class/rtc/rtc0/wakealarm`) or by wrapping `/usr/sbin/rtcwake` as a sub-process, and how should local time vs UTC epoch conversions be handled across diverse Linux kernel and motherboard BIOS implementations?

## Answer

Arkon should program the hardware wakealarm via **direct sysfs writes** to `/sys/class/rtc/rtc0/wakealarm` using relative second syntax (`+<seconds>`) preceded by an explicit alarm cancellation (`0\n`), and must **not** wrap `/usr/sbin/rtcwake`.

Key architecture conclusions:
1. **Direct Sysfs vs `rtcwake`**: Direct sysfs writes preserve Arkon's single static binary invariant with zero runtime dependencies on `util-linux`, avoid process fork/exec overhead, provide clean integer epoch status inspection for `arkon status` (avoiding fragile `ctime(3)` scraping), and critically prevent `rtcwake` from hijacking the host poweroff before Arkon's Docker Compose profile teardown pipeline finishes.
2. **UTC vs Local Time in BIOS**: Programming with relative syntax (`+<seconds>`) completely bypasses the timezone problem during write operations because the kernel (`drivers/rtc/sysfs.c`) computes `alarm = now + offset` using the hardware RTC's internal counter, making writes completely invariant to BIOS timezone. For status inspection, Arkon reads `/etc/adjtime` line 3 (`UTC` vs `LOCAL`) and maps the readback integer to local wall-clock time accordingly.
3. **Capabilities & Permissions**: Sysfs wakealarm does not check `CAP_WAKE_ALARM` (which is only checked for POSIX alarmtimers); it is governed by standard DAC permissions (`0644 root:root`), requiring `root` or `CAP_DAC_OVERRIDE` for writes, while reads (`arkon status`) are completely unprivileged.
4. **Validation & Quirks**:
   - Always write `0\n` first to avoid the kernel `-EBUSY` condition when an alarm is already pending.
   - Enforce a minimum safety buffer of 180 seconds before poweroff to avoid the S5 shutdown race condition.
   - Read back `/sys/class/rtc/rtc0/wakealarm` to verify hardware retention before system poweroff.
   - Require ErP/EuP mode to be disabled in BIOS setup so the ATX `+5VSB` rail maintains power to the ACPI RTC wake logic.
   - Detect 24-hour hardware comparator limits on older boards lacking ACPI FADT `DAY_ALRM`.

Detailed investigation and Go package blueprint are documented in [`.scratch/arkon/research/02-rtc-wake-and-acpi-s5-abstraction.md`](file:///home/adrian/adrian-dev/arkon/.scratch/arkon/research/02-rtc-wake-and-acpi-s5-abstraction.md).
