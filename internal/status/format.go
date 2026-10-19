package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/state"
)

// FormatStatus renders the diagnostic data model into a borderless left-rail layout.
func FormatStatus(data *StatusData, noColor bool) string {
	if data == nil {
		return ""
	}

	var (
		bold   = "\033[1m"
		reset  = "\033[0m"
		green  = "\033[38;5;46m"
		yellow = "\033[38;5;220m"
		gray   = "\033[38;5;244m"
	)
	if noColor {
		bold = ""
		reset = ""
		green = ""
		yellow = ""
		gray = ""
	}

	var sb strings.Builder

	// ARKON HOST STATUS
	badge := "[ACPI S0 Operational]"
	if data.HostPowerState != "" {
		badge = fmt.Sprintf("[%s]", data.HostPowerState)
	}
	sb.WriteString(fmt.Sprintf("%s● ARKON HOST STATUS%s %s%s%s\n", bold, reset, green, badge, reset))
	if data.CPUArch != "" {
		sb.WriteString(fmt.Sprintf("  Host:        %s (%s)\n", data.Hostname, data.CPUArch))
	} else {
		sb.WriteString(fmt.Sprintf("  Host:        %s\n", data.Hostname))
	}
	sb.WriteString(fmt.Sprintf("  Timezone:    %s\n\n", data.Timezone))

	if data.DriftNotice != "" {
		sb.WriteString(fmt.Sprintf("%s● Active Manifest Notice%s\n", bold, reset))
		sb.WriteString(fmt.Sprintf("  %sNotice:      %s%s\n\n", yellow, data.DriftNotice, reset))
	}

	// Circadian Schedule
	if data.ManifestError != "" {
		sb.WriteString(fmt.Sprintf("%s● Circadian Schedule%s %s[BROKEN ACTIVE MANIFEST]%s\n", bold, reset, yellow, reset))
		if data.ActiveManifest != "" {
			sb.WriteString(fmt.Sprintf("  Path:        %s\n", data.ActiveManifest))
		}
		sb.WriteString("  Status:      Active manifest file is missing or unreadable\n")
		sb.WriteString("  Action:      Restore the manifest or run 'sudo arkon apply -c <path>' to reconfigure\n\n")
	} else if !data.Configured && !data.CircadianTimer.Configured && data.CircadianTimer.SleepAt == "" && data.CircadianTimer.WakeAt == "" {
		sb.WriteString(fmt.Sprintf("%s● Circadian Schedule%s %s[NOT CONFIGURED]%s\n", bold, reset, gray, reset))
		sb.WriteString("  Status:      No active manifest applied to this host\n")
		sb.WriteString("  Action:      Run 'sudo arkon apply -c <path>' to configure host lifecycle\n\n")
	} else {
		timerState := "inactive"
		if data.CircadianTimer.Active {
			timerState = "active"
		}
		timerUnit := data.CircadianTimer.TimerUnit
		if timerUnit == "" {
			timerUnit = "arkon-sleep.timer"
		}
		sb.WriteString(fmt.Sprintf("%s● Circadian Schedule%s %s(%s %s)%s\n", bold, reset, gray, timerUnit, timerState, reset))
		if data.ActiveManifest != "" {
			sb.WriteString(fmt.Sprintf("  Manifest:    %s\n", data.ActiveManifest))
		}
		if data.CircadianTimer.SleepAt != "" || data.CircadianTimer.WakeAt != "" {
			sb.WriteString(fmt.Sprintf("  Schedule:    Sleep @ %s%s%s  |  Wake @ %s%s%s\n",
				yellow, data.CircadianTimer.SleepAt, reset,
				green, data.CircadianTimer.WakeAt, reset))
		} else {
			sb.WriteString("  Schedule:    Disabled\n")
		}
		if data.CircadianTimer.NextTime != "" {
			sb.WriteString(fmt.Sprintf("  Next Action: %s, %s\n\n", data.CircadianTimer.NextAction, data.CircadianTimer.NextTime))
		} else if data.CircadianTimer.NextAction != "" {
			sb.WriteString(fmt.Sprintf("  Next Action: %s\n\n", data.CircadianTimer.NextAction))
		} else {
			sb.WriteString("  Next Action: None\n\n")
		}
	}

	// Hardware RTC Wakealarm
	rtcBadge := "[DISABLED]"
	rtcColor := gray
	if data.RTCWakeAlarm != nil && data.RTCWakeAlarm.Enabled {
		rtcBadge = "[ARMED]"
		rtcColor = green
	}
	sb.WriteString(fmt.Sprintf("%s● Hardware RTC Wakealarm%s %s%s%s\n", bold, reset, rtcColor, rtcBadge, reset))
	dev := "/sys/class/rtc/rtc0"
	if data.RTCWakeAlarm != nil && data.RTCWakeAlarm.Device != "" {
		dev = data.RTCWakeAlarm.Device
	}
	sb.WriteString(fmt.Sprintf("  Device:      %s\n", dev))
	if data.RTCWakeAlarm != nil && data.RTCWakeAlarm.Enabled {
		if data.RTCWakeAlarm.SecondsUntil > 0 {
			rem := state.FormatRemaining(time.Duration(data.RTCWakeAlarm.SecondsUntil) * time.Second)
			sb.WriteString(fmt.Sprintf("  Target:      %s %s(%s remaining)%s\n",
				data.RTCWakeAlarm.AlarmTimeHost, gray, rem, reset))
		} else if data.RTCWakeAlarm.AlarmTimeHost != "" {
			sb.WriteString(fmt.Sprintf("  Target:      %s\n", data.RTCWakeAlarm.AlarmTimeHost))
		} else {
			sb.WriteString("  Target:      Armed\n")
		}
	} else {
		sb.WriteString("  Target:      None scheduled\n")
	}
	clockModeStr := "UTC"
	if data.RTCWakeAlarm != nil && data.RTCWakeAlarm.ClockMode != "" {
		clockModeStr = data.RTCWakeAlarm.ClockMode
	}
	sb.WriteString(fmt.Sprintf("  Clock Mode:  /etc/adjtime (%s)\n", clockModeStr))
	if data.RTCWakeAlarm != nil && data.RTCWakeAlarm.KernelDirty != "" {
		sb.WriteString(fmt.Sprintf("  Kernel Dirty: %s%s%s %s(flushed to disk)%s\n",
			green, data.RTCWakeAlarm.KernelDirty, reset, gray, reset))
	}
	sb.WriteString("\n")

	// Workload Teardown Pipeline
	sb.WriteString(fmt.Sprintf("%s● Workload Teardown Pipeline%s\n", bold, reset))
	provider := data.Teardown.Provider
	if provider == "" {
		provider = "docker-compose"
	}
	stackWord := "stacks"
	if data.Teardown.StackCount == 1 {
		stackWord = "stack"
	}
	sb.WriteString(fmt.Sprintf("  Provider:    %s (%d %s)\n", provider, data.Teardown.StackCount, stackWord))
	if len(data.Teardown.ProfileOrder) > 0 {
		sb.WriteString(fmt.Sprintf("  Progression: %s → syscall.Sync()\n", strings.Join(data.Teardown.ProfileOrder, " → ")))
	} else {
		sb.WriteString("  Progression: syscall.Sync()\n")
	}
	if len(data.Teardown.Stacks) > 0 {
		sb.WriteString(fmt.Sprintf("  Stacks:      %s\n", strings.Join(data.Teardown.Stacks, ", ")))
	}
	sb.WriteString("\n")

	// Workload Startup Pipeline
	bootUnit := data.Startup.BootUnit
	if bootUnit == "" {
		bootUnit = "inactive"
	}
	sb.WriteString(fmt.Sprintf("%s● Workload Startup Pipeline%s %s(%s %s)%s\n", bold, reset, gray, "arkon-boot.service", bootUnit, reset))
	startupProvider := data.Startup.Provider
	if startupProvider == "" {
		startupProvider = "docker-compose"
	}
	startupStackWord := "stacks"
	if data.Startup.StackCount == 1 {
		startupStackWord = "stack"
	}
	sb.WriteString(fmt.Sprintf("  Provider:    %s (%d %s)\n", startupProvider, data.Startup.StackCount, startupStackWord))
	var progParts []string
	if len(data.Hooks.PostWake) > 0 {
		progParts = append(progParts, "post_wake hooks")
	}
	progParts = append(progParts, data.Startup.Profiles...)
	if len(progParts) > 0 {
		sb.WriteString(fmt.Sprintf("  Progression: %s\n", strings.Join(progParts, " → ")))
	} else {
		sb.WriteString("  Progression: None\n")
	}
	if len(data.Startup.Stacks) > 0 {
		sb.WriteString(fmt.Sprintf("  Stacks:      %s\n", strings.Join(data.Startup.Stacks, ", ")))
	}
	sb.WriteString("\n")

	// Lifecycle Hooks
	sb.WriteString(fmt.Sprintf("%s● Lifecycle Hooks%s\n", bold, reset))
	hasHooks := len(data.Hooks.PreTeardown) > 0 ||
		len(data.Hooks.PostTeardown) > 0 ||
		len(data.Hooks.PreSleep) > 0 ||
		len(data.Hooks.PostWake) > 0
	if !hasHooks {
		sb.WriteString("  Hooks: None configured\n\n")
	} else {
		if len(data.Hooks.PreTeardown) > 0 {
			sb.WriteString(fmt.Sprintf("  Pre-Teardown:  %s\n", strings.Join(data.Hooks.PreTeardown, ", ")))
		}
		if len(data.Hooks.PostTeardown) > 0 {
			sb.WriteString(fmt.Sprintf("  Post-Teardown: %s\n", strings.Join(data.Hooks.PostTeardown, ", ")))
		}
		if len(data.Hooks.PreSleep) > 0 {
			sb.WriteString(fmt.Sprintf("  Pre-Sleep:     %s\n", strings.Join(data.Hooks.PreSleep, ", ")))
		}
		if len(data.Hooks.PostWake) > 0 {
			sb.WriteString(fmt.Sprintf("  Post-Wake:     %s\n", strings.Join(data.Hooks.PostWake, ", ")))
		}
		sb.WriteString("\n")
	}

	// Maintenance Locks
	sb.WriteString(fmt.Sprintf("%s● Maintenance Locks%s\n", bold, reset))
	if len(data.InhibitionLocks) == 0 {
		sb.WriteString(fmt.Sprintf("  Inhibitions: %sNone active%s (sleep will trigger as scheduled)\n\n", green, reset))
	} else {
		for _, l := range data.InhibitionLocks {
			rem := l.Remaining
			if rem == "" && !l.ExpiresAt.IsZero() {
				rem = state.FormatRemaining(time.Until(l.ExpiresAt))
			}
			if rem != "" {
				sb.WriteString(fmt.Sprintf("  - %s%s%s: \"%s\" %s(expires in %s)%s\n",
					yellow, l.ID, reset, l.Reason, gray, rem, reset))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s%s%s: \"%s\"\n", yellow, l.ID, reset, l.Reason))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
