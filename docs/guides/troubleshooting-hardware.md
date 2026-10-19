# Hardware RTC & Power Troubleshooting

This runbook diagnoses issues with hardware Real-Time Clock (RTC) wakealarms and ACPI power transitions across diverse PC and server motherboards.

---

## 1. Verifying Kernel RTC Sysfs Access

Arkon communicates directly with the Linux kernel RTC subsystem. Check that your kernel exposes the standard sysfs interface:

```bash
ls -l /sys/class/rtc/rtc0/wakealarm
```

If the file exists, check whether an alarm is currently armed:

```bash
cat /sys/class/rtc/rtc0/wakealarm
```

- An empty return means no wakealarm is currently armed.
- An integer return (such as `1773950400`) represents the target wake epoch in UNIX seconds.

If `/sys/class/rtc/rtc0/wakealarm` does not exist:
1. Verify the `rtc_cmos` module is loaded:
   ```bash
   lsmod | grep rtc
   ```
2. Check kernel boot logs for RTC device detection:
   ```bash
   dmesg | grep -i rtc
   ```
   A healthy boot will log: `rtc_cmos 00:02: RTC can wake from S4` or similar.

---

## 2. Reading Low-Level Hardware Alarm Flags

For low-level hardware diagnostics, read `/proc/driver/rtc`:

```bash
cat /proc/driver/rtc
```

Expected output on a healthy system:

```text
rtc_time        : 16:45:12
rtc_date        : 2026-10-19
alrm_time       : 01:00:00
alrm_date       : 2026-10-20
alarm_IRQ       : yes
alrm_pending    : no
update_IRQ      : no
periodic_IRQ    : no
periodic_freq   : 1024
batt_status     : okay
```

Key indicators:
- `alarm_IRQ: yes`: Indicates the motherboard hardware interrupt line is actively armed to trigger a wake event.
- `batt_status: okay`: The onboard CMOS coin-cell battery has sufficient voltage. If this reports `dead`, the motherboard cannot retain alarm state when main wall power fluctuates.

---

## 3. Testing Wakealarm Manually (3-Minute Test)

You can verify hardware wake functionality without running a full Arkon sleep cycle:

```bash
# 1. Clear any existing alarm
echo 0 | sudo tee /sys/class/rtc/rtc0/wakealarm

# 2. Arm a test alarm 180 seconds into the future
echo +180 | sudo tee /sys/class/rtc/rtc0/wakealarm

# 3. Verify retention readback
cat /sys/class/rtc/rtc0/wakealarm

# 4. Put host into S5 poweroff
sudo systemctl poweroff
```

Wait 3 minutes. If the machine powers on automatically, your motherboard BIOS and RTC hardware are completely compatible with ACPI S5 wake.

---

## 4. Clock Mode Drift: UTC vs LOCAL

Linux supports two RTC clock modes configured in `/etc/adjtime`:
- **UTC (Recommended):** The hardware clock stores UTC time. Operating systems handle local timezone offsets in software.
- **LOCAL:** The hardware clock stores local wall-clock time (common in dual-boot Windows setups).

Arkon reads `/etc/adjtime` automatically. If your hardware clock is set to local time or drift occurs, check `timedatectl`:

```bash
timedatectl status
```

Ensure:
- `System clock synchronized: yes`
- `RTC in local TZ: no`

If `RTC in local TZ` is set to `yes`, convert it back to standard UTC:

```bash
sudo timedatectl set-local-rtc 0
```

---

## 5. Motherboard BIOS Quirks & ACPI States

### ACPI S5 (Soft-Off) vs ACPI S3 (Suspend to RAM)
Most modern motherboards support waking from ACPI S5 (complete poweroff). However, some older laptop motherboards or budget OEM desktop BIOSes (older HP, Dell OptiPlex, or Lenovo ThinkCentre units) only support RTC alarms when waking from ACPI S3 (Suspend to RAM).

If your machine fails to wake from poweroff:
1. Open `arkon.yaml` and switch the power transition mode to suspend:
   ```yaml
   host:
     power_action: "suspend"
   ```
2. Re-apply configuration:
   ```bash
   sudo arkon apply
   ```

### BIOS Power Settings Checklist
If the machine still does not wake automatically:
1. Enter BIOS / UEFI setup at boot (typically `Del` or `F2`).
2. Navigate to **Power Management** or **Advanced ACPI Configuration**.
3. Ensure **ACPI RTC Alarm Resume** or **Wake on RTC** is enabled (or set to `By OS` rather than `By BIOS`).
4. Disable **Deep Sleep** or **ErP / EuP Ready**. When ErP is set to `S4+S5`, motherboards cut standby power completely to the motherboard bus, disabling the RTC interrupt pin.
5. Check the CMOS battery (CR2032). If your system loses the current date and time whenever unplugged from the wall, replace the CR2032 coin-cell battery.
