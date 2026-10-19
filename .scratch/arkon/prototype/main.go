// PROTOTYPE ONLY - Throwaway code for evaluating Arkon Terminal UX & CLI contracts.
// See .scratch/arkon/issues/05-cli-command-surface-and-terminal-ux.md
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI Styling Constants (Clean, lightweight, borderless)
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	green  = "\033[38;5;46m"
	yellow = "\033[38;5;220m"
	cyan   = "\033[38;5;51m"
	white  = "\033[38;5;231m"
	gray   = "\033[38;5;244m"
	red    = "\033[38;5;196m"
)

type Envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type StatusData struct {
	HostPowerState  string            `json:"host_power_state"`
	Hostname        string            `json:"hostname"`
	CPUArch         string            `json:"cpu_arch"`
	Timezone        string            `json:"timezone"`
	RTCWakeAlarm    *RTCStatus        `json:"rtc_wake_alarm"`
	CircadianTimer  CircadianStatus   `json:"circadian_timer"`
	InhibitionLocks []InhibitionLock  `json:"inhibition_locks"`
	Teardown        TeardownStatus    `json:"teardown"`
}

type RTCStatus struct {
	Enabled       bool   `json:"enabled"`
	AlarmTimeUTC  string `json:"alarm_time_utc,omitempty"`
	AlarmTimeHost string `json:"alarm_time_host,omitempty"`
	SecondsUntil  int64  `json:"seconds_until,omitempty"`
	Device        string `json:"device"`
}

type CircadianStatus struct {
	SleepAt    string `json:"sleep_at"`
	WakeAt     string `json:"wake_at"`
	NextAction string `json:"next_action"`
	NextTime   string `json:"next_time"`
	TimerUnit  string `json:"timer_unit"`
	Active     bool   `json:"active"`
}

type InhibitionLock struct {
	ID        string `json:"id"`
	Reason    string `json:"reason"`
	CreatedBy string `json:"created_by"`
	ExpiresAt string `json:"expires_at"`
	Remaining string `json:"remaining"`
}

type TeardownStatus struct {
	Provider     string   `json:"provider"`
	ProfileOrder []string `json:"profile_order"`
	StackCount   int      `json:"stack_count"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "status":
		runStatus(args)
	case "apply":
		runApply(args)
	case "inhibit":
		runInhibit(args)
	case "sleep":
		runSleep(args)
	case "schema":
		runSchema(args)
	case "version":
		fmt.Println("arkon-prototype v0.1.0-dev (interactive UX evaluation)")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "%sError:%s unknown command '%s'\n\n", red, reset, command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`%s%sArkon%s %sv0.1.0-dev%s: Host Power Lifecycle & Docker Orchestrator

%sUSAGE:%s
  arkon <command> [flags]

%sCOMMANDS:%s
  %sstatus%s     Display host power state, RTC alarm, schedule & locks
  %sapply%s      Compile arkon.yaml into verified /etc/systemd/system units
  %ssleep%s      Execute phased container teardown, sync disk & poweroff
  %sinhibit%s    Create or release temporary maintenance poweroff overrides
  %sschema%s     Emit canonical JSON Schema (Draft 2020-12) for manifests and state

%sFLAGS:%s
  %s--json%s          Output strictly-typed JSON envelope
  %s-c, --config%s    Path to arkon.yaml manifest (default: "./arkon.yaml")
  %s-h, --help%s      Show command help

%sEXIT CODES:%s
  0 = OK / Success
  1 = General / System Error
  2 = Power Transition Inhibited (Active Lock or Daytime Protection)
  3 = Manifest Validation / Compilation Failure
  4 = Insufficient Privileges (Root required without --dry-run)
`, bold, white, reset, gray, reset, bold, reset, bold, reset,
		cyan, reset, cyan, reset, cyan, reset, cyan, reset, cyan, reset,
		bold, reset, yellow, reset, yellow, reset, yellow, reset, bold, reset)
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output JSON")
	fs.Parse(args)

	now := time.Now()
	alarmTime := now.Add(7*time.Hour + 18*time.Minute)

	data := StatusData{
		HostPowerState: "ACPI S0 (Operational Active)",
		Hostname:       "komsos",
		CPUArch:        "Intel Pentium Dual-Core E5400 (x86_64)",
		Timezone:       "Asia/Jakarta (WIB, UTC+07:00)",
		RTCWakeAlarm: &RTCStatus{
			Enabled:       true,
			AlarmTimeUTC:  alarmTime.UTC().Format(time.RFC3339),
			AlarmTimeHost: alarmTime.Format("2006-01-02 15:04:05 MST"),
			SecondsUntil:  int64(alarmTime.Sub(now).Seconds()),
			Device:        "/sys/class/rtc/rtc0",
		},
		CircadianTimer: CircadianStatus{
			SleepAt:    "23:00",
			WakeAt:     "07:00",
			NextAction: "Sleep (ACPI S5 Soft-Off)",
			NextTime:   "Today at 23:00:00 WIB (in 5h 18m)",
			TimerUnit:  "arkon-sleep.timer",
			Active:     true,
		},
		InhibitionLocks: []InhibitionLock{},
		Teardown: TeardownStatus{
			Provider:     "docker-compose",
			ProfileOrder: []string{"apps", "infra"},
			StackCount:   1,
		},
	}

	if *jsonOutput {
		emitJSON(true, data, nil)
		return
	}

	fmt.Printf("%s%s● ARKON HOST STATUS%s %s[ACPI S0 Operational]%s\n", bold, white, reset, green, reset)
	fmt.Printf("  %sHost:%s        %s (%s)\n", gray, reset, data.Hostname, data.CPUArch)
	fmt.Printf("  %sTimezone:%s    %s\n\n", gray, reset, data.Timezone)

	fmt.Printf("%s%s● Circadian Schedule%s %s(%s active)%s\n", bold, cyan, reset, gray, data.CircadianTimer.TimerUnit, reset)
	fmt.Printf("  %sSchedule:%s    Sleep @ %s%s%s  |  Wake @ %s%s%s\n", gray, reset, yellow, data.CircadianTimer.SleepAt, reset, green, data.CircadianTimer.WakeAt, reset)
	fmt.Printf("  %sNext Action:%s Sleep today @ %s23:00:00 WIB%s %s(in 5h 18m)%s\n\n", gray, reset, yellow, reset, gray, reset)

	fmt.Printf("%s%s● Hardware RTC Wakealarm%s %s[ARMED]%s\n", bold, cyan, reset, green, reset)
	fmt.Printf("  %sDevice:%s      %s\n", gray, reset, data.RTCWakeAlarm.Device)
	fmt.Printf("  %sTarget:%s      %s %s(7h 18m remaining)%s\n", gray, reset, data.RTCWakeAlarm.AlarmTimeHost, gray, reset)
	fmt.Printf("  %sClock Mode:%s  /etc/adjtime (UTC invariant)\n", gray, reset)
	fmt.Printf("  %sKernel Dirty:%s %s0 kB%s %s(flushed to disk)%s\n\n", gray, reset, green, reset, gray, reset)

	fmt.Printf("%s%s● Workload Teardown Pipeline%s\n", bold, cyan, reset)
	fmt.Printf("  %sProvider:%s    %s (1 stack, 5 containers)\n", gray, reset, data.Teardown.Provider)
	fmt.Printf("  %sProgression:%s %s → syscall.Sync()\n", gray, reset, strings.Join(data.Teardown.ProfileOrder, " → "))
	fmt.Printf("  %sStacks:%s      /opt/stacks/apps\n\n", gray, reset)

	fmt.Printf("%s%s● Maintenance Locks%s\n", bold, cyan, reset)
	if len(data.InhibitionLocks) == 0 {
		fmt.Printf("  %sInhibitions:%s %sNone active%s (sleep will trigger as scheduled)\n\n", gray, reset, green, reset)
	} else {
		for _, l := range data.InhibitionLocks {
			fmt.Printf("  - %s%s%s: \"%s\" %s(expires in %s)%s\n", yellow, l.ID, reset, l.Reason, gray, l.Remaining, reset)
		}
		fmt.Println()
	}
}

func runApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Preview generated units without applying")
	jsonOutput := fs.Bool("json", false, "Output JSON")
	configPath := fs.String("config", "./arkon.yaml", "Manifest path")
	fs.StringVar(configPath, "c", "./arkon.yaml", "Manifest path (shorthand)")
	fs.Parse(args)

	isRoot := os.Geteuid() == 0

	if !isRoot && !*dryRun {
		if *jsonOutput {
			emitJSON(false, nil, &Error{
				Code:    "ERR_INSUFFICIENT_PERMISSIONS",
				Message: "Applying systemd units requires root privileges (run with sudo or pass --dry-run)",
			})
			os.Exit(4)
		}
		fmt.Fprintf(os.Stderr, "%s● ERROR (code 4):%s Root privileges required to write /etc/systemd/system/\n", red, reset)
		fmt.Fprintf(os.Stderr, "  Arkon requires elevated access to install systemd service & timer units.\n")
		fmt.Fprintf(os.Stderr, "  • Apply with sudo:      %ssudo arkon apply%s\n", cyan, reset)
		fmt.Fprintf(os.Stderr, "  • Inspect without root: %sarkon apply --dry-run%s\n\n", yellow, reset)
		os.Exit(4)
	}

	result := map[string]any{
		"manifest":    *configPath,
		"dry_run":     *dryRun,
		"validated":   true,
		"units": []string{
			"/etc/systemd/system/arkon-sleep.service",
			"/etc/systemd/system/arkon-sleep.timer",
		},
		"systemd_reload": !*dryRun,
		"timer_enabled":  !*dryRun,
		"next_trigger":   "23:00:00 Asia/Jakarta",
	}

	if *jsonOutput {
		emitJSON(true, result, nil)
		return
	}

	fmt.Printf("%s● ARKON SYSTEMD COMPILER%s\n", bold, reset)
	fmt.Printf("  %s✔ [1/4]%s Manifest parsed: %s%s%s (version: 1, provider: docker-compose)\n", green, reset, cyan, *configPath, reset)
	fmt.Printf("  %s✔ [2/4]%s Compiled units: /etc/systemd/system/arkon-sleep.{service,timer}\n", green, reset)
	fmt.Printf("  %s✔ [3/4]%s Verified syntax: systemd-analyze verify (0 errors, 0 warnings)\n", green, reset)

	if *dryRun {
		fmt.Printf("\n  %sDRY RUN COMPLETE:%s Units validated cleanly. No changes written to disk.\n\n", yellow, reset)
		return
	}

	fmt.Printf("  %s✔ [4/4]%s Activated units: systemctl daemon-reload && systemctl enable --now arkon-sleep.timer\n", green, reset)
	fmt.Printf("\n  %s✔ Installation successful!%s Next poweroff: %sToday @ 23:00:00 WIB%s\n\n", green, reset, yellow, reset)
}

func runInhibit(args []string) {
	fs := flag.NewFlagSet("inhibit", flag.ExitOnError)
	duration := fs.String("d", "2h", "Duration of lock")
	fs.StringVar(duration, "duration", "2h", "Duration of lock")
	reason := fs.String("r", "Manual maintenance window", "Reason for inhibition lock")
	fs.StringVar(reason, "reason", "Manual maintenance window", "Reason for inhibition lock")
	release := fs.Bool("release", false, "Release/clear active lock")
	fs.BoolVar(release, "clear", false, "Clear all locks (shorthand)")
	list := fs.Bool("list", false, "List active locks")
	jsonOutput := fs.Bool("json", false, "Output JSON")
	fs.Parse(args)

	if *list {
		if *jsonOutput {
			emitJSON(true, []InhibitionLock{}, nil)
			return
		}
		fmt.Printf("● Maintenance Locks: None active (operational)\n\n")
		return
	}

	if *release {
		if *jsonOutput {
			emitJSON(true, map[string]string{"status": "released", "cleared_locks": "all"}, nil)
			return
		}
		fmt.Printf("%s✔ Maintenance locks released.%s Circadian schedule resumed.\n\n", green, reset)
		return
	}

	lockID := fmt.Sprintf("inh-%s", time.Now().Format("20060102-150405"))
	exp := time.Now().Add(2 * time.Hour)

	lock := InhibitionLock{
		ID:        lockID,
		Reason:    *reason,
		CreatedBy: "operator (UID 1000)",
		ExpiresAt: exp.Format(time.RFC3339),
		Remaining: *duration,
	}

	if *jsonOutput {
		emitJSON(true, lock, nil)
		return
	}

	fmt.Printf("%s● Inhibition Lock Created:%s %s%s%s\n", yellow, reset, cyan, lock.ID, reset)
	fmt.Printf("  Reason:    %s\n", lock.Reason)
	fmt.Printf("  Duration:  %s (expires at %s)\n", *duration, exp.Format("15:04:05 MST"))
	fmt.Printf("  To cancel: %sarkon inhibit --release%s\n\n", cyan, reset)
}

func runSleep(args []string) {
	fs := flag.NewFlagSet("sleep", flag.ExitOnError)
	force := fs.Bool("force", false, "Force sleep, overriding daytime and active locks")
	jsonOutput := fs.Bool("json", false, "Output JSON")
	simulateInhibit := fs.Bool("simulate-inhibit", false, "Simulate an active lock blocking sleep")
	simulateDaytime := fs.Bool("simulate-daytime", false, "Simulate daytime protection violation")
	fs.Parse(args)

	if *simulateDaytime && !*force {
		errObj := &Error{
			Code:    "ERR_DAYTIME_PROTECTION",
			Message: "Refusing sleep: current time 14:30 is within daytime operational window (wake_at: 07:00, sleep_at: 23:00). Pass --force to override.",
		}
		if *jsonOutput {
			emitJSON(false, nil, errObj)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "%s● ERROR (code 2): Refusing sleep during daytime window%s\n", red, reset)
		fmt.Fprintf(os.Stderr, "  Current time (14:30 WIB) is within operational active hours (07:00 - 23:00).\n")
		fmt.Fprintf(os.Stderr, "  To override: %sarkon sleep --force%s\n\n", cyan, reset)
		os.Exit(2)
	}

	if *simulateInhibit && !*force {
		errObj := &Error{
			Code:    "ERR_INHIBITED",
			Message: "Sleep transition inhibited by active lock 'inh-20261018-01' (\"Running backup\"). Release with 'arkon inhibit --release' or override with '--force'.",
		}
		if *jsonOutput {
			emitJSON(false, nil, errObj)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "%s● INHIBITED (code 2): Active maintenance lock detected%s\n", yellow, reset)
		fmt.Fprintf(os.Stderr, "  Lock ID:     %s%sinh-20261018-01%s\n", cyan, reset, reset)
		fmt.Fprintf(os.Stderr, "  Reason:      Running database backup & sync\n")
		fmt.Fprintf(os.Stderr, "  Expires:     in 1h 14m (at 01:30:00 WIB)\n")
		fmt.Fprintf(os.Stderr, "  To release:  %sarkon inhibit --release%s\n", cyan, reset)
		fmt.Fprintf(os.Stderr, "  To override: %sarkon sleep --force%s\n\n", cyan, reset)
		os.Exit(2)
	}

	if *jsonOutput {
		emitJSON(true, map[string]any{
			"status":          "completed",
			"tiers_stopped":   []string{"apps", "infra"},
			"disk_synced":     true,
			"wakealarm_armed": true,
			"power_action":    "poweroff",
		}, nil)
		return
	}

	fmt.Printf("%s● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE%s\n\n", bold, reset)
	fmt.Printf("  %s✔ [1/5]%s Invariants confirmed (0 active locks, daytime clear)\n", green, reset)

	fmt.Printf("  %s▼ [2/5]%s Teardown Tier 1: profile 'apps'\n", cyan, reset)
	time.Sleep(200 * time.Millisecond)
	fmt.Printf("        ├─ %s✔ nextcloud%s   [exited 0] (0.8s)\n", green, reset)
	fmt.Printf("        ├─ %s✔ jellyfin%s    [exited 0] (0.6s)\n", green, reset)
	fmt.Printf("        └─ %s✔ vaultwarden%s [exited 0] (0.2s)\n", green, reset)

	fmt.Printf("  %s▼ [3/5]%s Teardown Tier 2: profile 'infra'\n", cyan, reset)
	time.Sleep(300 * time.Millisecond)
	fmt.Printf("        ├─ %s✔ postgres%s    [WAL checkpoint complete | clean exit] (1.4s)\n", green, reset)
	fmt.Printf("        └─ %s✔ redis%s       [RDB saved to disk | clean exit] (0.3s)\n", green, reset)

	fmt.Printf("  %s▼ [4/5]%s Storage Sync: syscall.Sync() issued\n", cyan, reset)
	fmt.Printf("        └─ /proc/meminfo Dirty: 142 MB → %s0 kB%s (flushed to physical media)\n", green, reset)

	fmt.Printf("  %s▼ [5/5]%s Hardware Wakealarm: /sys/class/rtc/rtc0/wakealarm\n", cyan, reset)
	fmt.Printf("        └─ %s✔ Armed for tomorrow @ 07:00:00 WIB%s (+28800s relative write)\n", green, reset)

	fmt.Printf("\n  %s✔ Teardown complete.%s Host entering ACPI S5 poweroff.\n\n", green, reset)
}

func runSchema(args []string) {
	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   "ArkonManifest",
		"type":    "object",
		"required": []string{"version", "host", "circadian", "teardown"},
		"properties": map[string]any{
			"version": map[string]any{
				"type": "string",
				"enum": []string{"1"},
			},
			"host": map[string]any{
				"type": "object",
				"required": []string{"timezone"},
				"properties": map[string]any{
					"timezone": map[string]any{"type": "string"},
					"rtc_device": map[string]any{
						"type":    "string",
						"default": "/sys/class/rtc/rtc0",
					},
					"power_action": map[string]any{
						"type":    "string",
						"enum":    []string{"poweroff", "suspend", "hibernate"},
						"default": "poweroff",
					},
				},
			},
			"circadian": map[string]any{
				"type": "object",
				"required": []string{"sleep_at", "wake_at"},
				"properties": map[string]any{
					"sleep_at": map[string]any{"type": "string", "pattern": "^[0-2][0-9]:[0-5][0-9]$"},
					"wake_at":  map[string]any{"type": "string", "pattern": "^[0-2][0-9]:[0-5][0-9]$"},
					"days":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"teardown": map[string]any{
				"type": "object",
				"required": []string{"profile_order"},
				"properties": map[string]any{
					"provider": map[string]any{
						"type":    "string",
						"enum":    []string{"docker-compose", "podman-compose", "command", "systemd"},
						"default": "docker-compose",
					},
					"timeout_seconds": map[string]any{"type": "integer", "default": 30},
					"profile_order":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(schema)
}

func emitJSON(success bool, data any, err *Error) {
	env := Envelope{
		Success: success,
		Data:    data,
		Error:   err,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(env)
}
