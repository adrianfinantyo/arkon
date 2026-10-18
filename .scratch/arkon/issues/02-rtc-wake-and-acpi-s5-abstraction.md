# [02] Linux RTC Wake & ACPI S5 Abstraction

Type: research
Status: open
Blocked by: none

## Question

Should Arkon program the hardware wakealarm via direct sysfs writes (`/sys/class/rtc/rtc0/wakealarm`) or by wrapping `/usr/sbin/rtcwake` as a sub-process, and how should local time vs UTC epoch conversions be handled across diverse Linux kernel and motherboard BIOS implementations?
