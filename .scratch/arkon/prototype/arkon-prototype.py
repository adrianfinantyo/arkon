#!/usr/bin/env python3
# PROTOTYPE ONLY - Throwaway code for evaluating Arkon Terminal UX & CLI contracts.
# See .scratch/arkon/issues/05-cli-command-surface-and-terminal-ux.md

import sys
import json
import time
import os
from datetime import datetime, timedelta

# ANSI colors - clean, lightweight styling without decorative box borders
RESET   = "\033[0m"
BOLD    = "\033[1m"
DIM     = "\033[2m"

GREEN   = "\033[38;5;46m"
YELLOW  = "\033[38;5;220m"
CYAN    = "\033[38;5;51m"
WHITE   = "\033[38;5;231m"
GRAY    = "\033[38;5;244m"
RED     = "\033[38;5;196m"

# Disable color if NO_COLOR is set or non-TTY
if os.environ.get("NO_COLOR") or not sys.stdout.isatty():
    RESET = BOLD = DIM = GREEN = YELLOW = CYAN = WHITE = GRAY = RED = ""

def emit_json(success, data=None, error=None):
    envelope = {
        "success": success,
        "data": data,
        "error": error
    }
    print(json.dumps(envelope, indent=2))

def print_usage():
    print(f"""{BOLD}{WHITE}Arkon{RESET} {GRAY}v0.1.0-dev{RESET} — Host Power Lifecycle & Docker Orchestrator

{BOLD}USAGE:{RESET}
  arkon <command> [flags]

{BOLD}COMMANDS:{RESET}
  {CYAN}status{RESET}     Display host power state, RTC alarm, schedule & locks
  {CYAN}apply{RESET}      Compile arkon.yaml into verified /etc/systemd/system units
  {CYAN}sleep{RESET}      Execute phased container teardown, sync disk & poweroff
  {CYAN}inhibit{RESET}    Create or release temporary maintenance poweroff overrides
  {CYAN}schema{RESET}     Emit canonical JSON Schema (Draft 2020-12) for manifests and state

{BOLD}FLAGS:{RESET}
  {YELLOW}--json{RESET}          Output strictly-typed JSON envelope
  {YELLOW}-c, --config{RESET}    Path to arkon.yaml manifest (default: "./arkon.yaml")
  {YELLOW}-h, --help{RESET}      Show command help

{BOLD}EXIT CODES:{RESET}
  0 = OK / Success
  1 = General / System Error
  2 = Power Transition Inhibited (Active Lock or Daytime Protection)
  3 = Manifest Validation / Compilation Failure
  4 = Insufficient Privileges (Root required without --dry-run)
""")

def run_status(args):
    json_mode = "--json" in args
    
    now = datetime.now()
    alarm_time = now + timedelta(hours=7, minutes=18)
    
    data = {
        "host_power_state": "ACPI S0 (Operational Active)",
        "hostname": "komsos",
        "cpu_arch": "Intel Pentium Dual-Core E5400 (x86_64)",
        "memory": "3.8 GiB DDR2",
        "timezone": "Asia/Jakarta (WIB, UTC+07:00)",
        "rtc_wake_alarm": {
            "enabled": True,
            "alarm_time_utc": alarm_time.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "alarm_time_host": alarm_time.strftime("%Y-%m-%d %H:%M:%S WIB"),
            "seconds_until": int((alarm_time - now).total_seconds()),
            "device": "/sys/class/rtc/rtc0"
        },
        "circadian_timer": {
            "sleep_at": "23:00",
            "wake_at": "07:00",
            "next_action": "Sleep (ACPI S5 Soft-Off)",
            "next_time": "Today at 23:00:00 WIB (in 5h 18m)",
            "timer_unit": "arkon-sleep.timer",
            "active": True
        },
        "inhibition_locks": [],
        "teardown": {
            "provider": "docker-compose",
            "profile_order": ["apps", "infra"],
            "stack_count": 1,
            "active_containers": 5
        }
    }
    
    if json_mode:
        emit_json(True, data=data)
        return

    # Single, lightweight, non-wrapping layout that adapts to any window size
    print(f"{BOLD}{WHITE}● ARKON HOST STATUS{RESET} {GREEN}[ACPI S0 Operational]{RESET}")
    print(f"  {GRAY}Host:{RESET}        {data['hostname']} ({data['cpu_arch']})")
    print(f"  {GRAY}Timezone:{RESET}    {data['timezone']}")
    print()
    print(f"{BOLD}{CYAN}● Circadian Schedule{RESET} {GRAY}({data['circadian_timer']['timer_unit']} active){RESET}")
    print(f"  {GRAY}Schedule:{RESET}    Sleep @ {YELLOW}{data['circadian_timer']['sleep_at']}{RESET}  |  Wake @ {GREEN}{data['circadian_timer']['wake_at']}{RESET}")
    print(f"  {GRAY}Next Action:{RESET} Sleep today @ {YELLOW}23:00:00 WIB{RESET} {GRAY}(in 5h 18m){RESET}")
    print()
    print(f"{BOLD}{CYAN}● Hardware RTC Wakealarm{RESET} {GREEN}[ARMED]{RESET}")
    print(f"  {GRAY}Device:{RESET}      {data['rtc_wake_alarm']['device']}")
    print(f"  {GRAY}Target:{RESET}      {data['rtc_wake_alarm']['alarm_time_host']} {GRAY}(7h 18m remaining){RESET}")
    print(f"  {GRAY}Clock Mode:{RESET}  /etc/adjtime (UTC invariant)")
    print(f"  {GRAY}Kernel Dirty:{RESET} {GREEN}0 kB{RESET} {GRAY}(flushed to disk){RESET}")
    print()
    print(f"{BOLD}{CYAN}● Workload Teardown Pipeline{RESET}")
    print(f"  {GRAY}Provider:{RESET}    {data['teardown']['provider']} (1 stack, 5 containers)")
    print(f"  {GRAY}Progression:{RESET} {' → '.join(data['teardown']['profile_order'])} → syscall.Sync()")
    print(f"  {GRAY}Stacks:{RESET}      /opt/stacks/apps")
    print()
    print(f"{BOLD}{CYAN}● Maintenance Locks{RESET}")
    if not data['inhibition_locks']:
        print(f"  {GRAY}Inhibitions:{RESET} {GREEN}None active{RESET} (sleep will trigger as scheduled)")
    else:
        for l in data['inhibition_locks']:
            print(f"  - {YELLOW}{l['id']}{RESET}: \"{l['reason']}\" {GRAY}(expires in {l['remaining']}){RESET}")
    print()

def run_apply(args):
    json_mode = "--json" in args
    dry_run = "--dry-run" in args
    is_root = os.geteuid() == 0
    
    if not is_root and not dry_run:
        err = {
            "code": "ERR_INSUFFICIENT_PERMISSIONS",
            "message": "Applying systemd units requires root privileges (run with sudo or pass --dry-run)"
        }
        if json_mode:
            emit_json(False, error=err)
            sys.exit(4)
        print(f"{RED}● ERROR (code 4):{RESET} Root privileges required to write /etc/systemd/system/", file=sys.stderr)
        print(f"  Arkon requires elevated access to install systemd service & timer units.", file=sys.stderr)
        print(f"  • Apply with sudo:      {CYAN}sudo arkon apply{RESET}", file=sys.stderr)
        print(f"  • Inspect without root: {YELLOW}arkon apply --dry-run{RESET}\n", file=sys.stderr)
        sys.exit(4)
        
    result = {
        "manifest": "./arkon.yaml",
        "dry_run": dry_run,
        "validated": True,
        "units": [
            "/etc/systemd/system/arkon-sleep.service",
            "/etc/systemd/system/arkon-sleep.timer"
        ],
        "systemd_reload": not dry_run,
        "timer_enabled": not dry_run,
        "next_trigger": "23:00:00 Asia/Jakarta"
    }
    
    if json_mode:
        emit_json(True, data=result)
        return

    print(f"{BOLD}● ARKON SYSTEMD COMPILER{RESET}")
    print(f"  {GREEN}✔ [1/4]{RESET} Manifest parsed: {CYAN}./arkon.yaml{RESET} (version: 1, provider: docker-compose)")
    print(f"  {GREEN}✔ [2/4]{RESET} Compiled units: /etc/systemd/system/arkon-sleep.{{service,timer}}")
    print(f"  {GREEN}✔ [3/4]{RESET} Verified syntax: systemd-analyze verify (0 errors, 0 warnings)")
    
    if dry_run:
        print(f"\n  {YELLOW}DRY RUN COMPLETE:{RESET} Units validated cleanly. No changes written to disk.\n")
        return

    print(f"  {GREEN}✔ [4/4]{RESET} Activated units: systemctl daemon-reload && systemctl enable --now arkon-sleep.timer")
    print(f"\n  {GREEN}✔ Installation successful!{RESET} Next poweroff: {YELLOW}Today @ 23:00:00 WIB{RESET}\n")

def run_inhibit(args):
    json_mode = "--json" in args
    is_release = "--release" in args or "--clear" in args
    is_list = "--list" in args
    
    if is_list:
        if json_mode:
            emit_json(True, data=[])
            return
        print(f"● Maintenance Locks: None active (operational)\n")
        return
        
    if is_release:
        if json_mode:
            emit_json(True, data={"status": "released", "cleared_locks": "all"})
            return
        print(f"{GREEN}✔ Maintenance locks released.{RESET} Circadian schedule resumed.\n")
        return

    duration = "2h"
    reason = "Manual maintenance window"
    
    for i, a in enumerate(args):
        if a in ("-d", "--duration") and i + 1 < len(args):
            duration = args[i+1]
        elif a in ("-r", "--reason") and i + 1 < len(args):
            reason = args[i+1]
            
    lock_id = f"inh-{datetime.now().strftime('%Y%m%d-%H%M%S')}"
    exp = datetime.now() + timedelta(hours=2)
    
    lock = {
        "id": lock_id,
        "reason": reason,
        "created_by": "operator (UID 1000)",
        "expires_at": exp.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "remaining": duration
    }
    
    if json_mode:
        emit_json(True, data=lock)
        return
        
    print(f"{YELLOW}● Inhibition Lock Created:{RESET} {CYAN}{lock['id']}{RESET}")
    print(f"  Reason:    {lock['reason']}")
    print(f"  Duration:  {duration} (expires at {exp.strftime('%H:%M:%S WIB')})")
    print(f"  To cancel: {CYAN}arkon inhibit --release{RESET}\n")

def run_sleep(args):
    json_mode = "--json" in args
    force = "--force" in args
    sim_inhibit = "--simulate-inhibit" in args
    sim_daytime = "--simulate-daytime" in args
    
    if sim_daytime and not force:
        err = {
            "code": "ERR_DAYTIME_PROTECTION",
            "message": "Refusing sleep: current time 14:30 is within daytime operational window (wake_at: 07:00, sleep_at: 23:00). Pass --force to override."
        }
        if json_mode:
            emit_json(False, error=err)
            sys.exit(2)
        print(f"{RED}● ERROR (code 2): Refusing sleep during daytime window{RESET}", file=sys.stderr)
        print(f"  Current time (14:30 WIB) is within operational active hours (07:00 - 23:00).", file=sys.stderr)
        print(f"  To override: {CYAN}arkon sleep --force{RESET}\n", file=sys.stderr)
        sys.exit(2)
        
    if sim_inhibit and not force:
        err = {
            "code": "ERR_INHIBITED",
            "message": "Sleep transition inhibited by active lock 'inh-20261018-01' (\"Running backup\"). Release with 'arkon inhibit --release' or override with '--force'."
        }
        if json_mode:
            emit_json(False, error=err)
            sys.exit(2)
        print(f"{YELLOW}● INHIBITED (code 2): Active maintenance lock detected{RESET}", file=sys.stderr)
        print(f"  Lock ID:     {CYAN}inh-20261018-01{RESET}", file=sys.stderr)
        print(f"  Reason:      Running database backup & sync", file=sys.stderr)
        print(f"  Expires:     in 1h 14m (at 01:30:00 WIB)", file=sys.stderr)
        print(f"  To release:  {CYAN}arkon inhibit --release{RESET}", file=sys.stderr)
        print(f"  To override: {CYAN}arkon sleep --force{RESET}\n", file=sys.stderr)
        sys.exit(2)

    if json_mode:
        emit_json(True, data={
            "status": "completed",
            "tiers_stopped": ["apps", "infra"],
            "disk_synced": True,
            "wakealarm_armed": True,
            "power_action": "poweroff"
        })
        return

    print(f"{BOLD}● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE{RESET}\n")
    print(f"  {GREEN}✔ [1/5]{RESET} Invariants confirmed (0 active locks, daytime clear)")
    
    print(f"  {CYAN}▼ [2/5]{RESET} Teardown Tier 1: profile 'apps'")
    time.sleep(0.2)
    print(f"        ├─ {GREEN}✔ nextcloud{RESET}   [exited 0] (0.8s)")
    print(f"        ├─ {GREEN}✔ jellyfin{RESET}    [exited 0] (0.6s)")
    print(f"        └─ {GREEN}✔ vaultwarden{RESET} [exited 0] (0.2s)")
    
    print(f"  {CYAN}▼ [3/5]{RESET} Teardown Tier 2: profile 'infra'")
    time.sleep(0.3)
    print(f"        ├─ {GREEN}✔ postgres{RESET}    [WAL checkpoint complete | clean exit] (1.4s)")
    print(f"        └─ {GREEN}✔ redis{RESET}       [RDB saved to disk | clean exit] (0.3s)")
    
    print(f"  {CYAN}▼ [4/5]{RESET} Storage Sync: syscall.Sync() issued")
    print(f"        └─ /proc/meminfo Dirty: 142 MB → {GREEN}0 kB{RESET} (flushed to physical media)")
    
    print(f"  {CYAN}▼ [5/5]{RESET} Hardware Wakealarm: /sys/class/rtc/rtc0/wakealarm")
    print(f"        └─ {GREEN}✔ Armed for tomorrow @ 07:00:00 WIB{RESET} (+28800s relative write)")
    
    print(f"\n  {GREEN}✔ Teardown complete.{RESET} Host entering ACPI S5 poweroff.\n")

def run_schema(args):
    schema = {
        "$schema": "https://json-schema.org/draft/2020-12/schema",
        "title": "ArkonManifest",
        "type": "object",
        "required": ["version", "host", "circadian", "teardown"],
        "properties": {
            "version": {"type": "string", "enum": ["1"]},
            "host": {
                "type": "object",
                "required": ["timezone"],
                "properties": {
                    "timezone": {"type": "string"},
                    "rtc_device": {"type": "string", "default": "/sys/class/rtc/rtc0"},
                    "power_action": {
                        "type": "string",
                        "enum": ["poweroff", "suspend", "hibernate"],
                        "default": "poweroff"
                    }
                }
            },
            "circadian": {
                "type": "object",
                "required": ["sleep_at", "wake_at"],
                "properties": {
                    "sleep_at": {"type": "string", "pattern": "^[0-2][0-9]:[0-5][0-9]$"},
                    "wake_at": {"type": "string", "pattern": "^[0-2][0-9]:[0-5][0-9]$"},
                    "days": {"type": "array", "items": {"type": "string"}}
                }
            },
            "teardown": {
                "type": "object",
                "required": ["profile_order"],
                "properties": {
                    "provider": {
                        "type": "string",
                        "enum": ["docker-compose", "podman-compose", "command", "systemd"],
                        "default": "docker-compose"
                    },
                    "timeout_seconds": {"type": "integer", "default": 30},
                    "profile_order": {"type": "array", "items": {"type": "string"}}
                }
            }
        }
    }
    print(json.dumps(schema, indent=2))

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print_usage()
        sys.exit(1)
        
    cmd = sys.argv[1]
    cmd_args = sys.argv[2:]
    
    if cmd == "status":
        run_status(cmd_args)
    elif cmd == "apply":
        run_apply(cmd_args)
    elif cmd == "inhibit":
        run_inhibit(cmd_args)
    elif cmd == "sleep":
        run_sleep(cmd_args)
    elif cmd == "schema":
        run_schema(cmd_args)
    elif cmd == "version":
        print("arkon-prototype v0.1.0-dev (interactive UX evaluation)")
    elif cmd in ("help", "-h", "--help"):
        print_usage()
    else:
        print(f"{RED}Error:{RESET} unknown command '{cmd}'\n", file=sys.stderr)
        print_usage()
        sys.exit(1)
