# [01] Declarative Manifest Schema & Systemd Compiler

Type: grilling
Status: open
Blocked by: none

## Question

How should the `arkon.yaml` manifest be structured to represent all host lifecycle parameters (schedules, timezone, daytime locks, teardown profiles, and pluggable hooks), and how should the compiler validate and write native `/etc/systemd/system/` timer and service units without leaving invalid unit syntax or broken state?
