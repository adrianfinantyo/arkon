package manifest

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var validPowerActions = map[string]bool{
	"poweroff":  true,
	"suspend":   true,
	"hibernate": true,
}

var validDays = map[string]bool{
	"Mon": true,
	"Tue": true,
	"Wed": true,
	"Thu": true,
	"Fri": true,
	"Sat": true,
	"Sun": true,
}

var validProviders = map[string]bool{
	"docker-compose": true,
	"podman-compose": true,
	"command":        true,
	"systemd":        true,
}

// MinimumSafetyBufferSeconds defines the minimum duration the host must remain asleep to avoid ACPI race conditions.
const MinimumSafetyBufferSeconds = 180

// Validate performs strict validation on a Manifest, returning a ValidationError with all collected issues.
func (m *Manifest) Validate() error {
	var errors []FieldError

	addErr := func(field, message string) {
		errors = append(errors, FieldError{Field: field, Message: message})
	}

	// 1. Version
	if m.Version != "1" {
		addErr("version", "version must be \"1\"")
	}

	// 2. Host
	if m.Host.Timezone == "" {
		addErr("host.timezone", "timezone is required")
	} else if _, err := time.LoadLocation(m.Host.Timezone); err != nil {
		addErr("host.timezone", fmt.Sprintf("timezone %q is not a valid IANA timezone in system tz database", m.Host.Timezone))
	}

	if m.Host.RTCDevice == "" {
		addErr("host.rtc_device", "rtc_device cannot be empty")
	}

	if !validPowerActions[m.Host.PowerAction] {
		addErr("host.power_action", fmt.Sprintf("invalid power_action %q, must be one of: poweroff, suspend, hibernate", m.Host.PowerAction))
	}

	// 3. Circadian
	sleepH, sleepM, sleepErr := ParseTimeHM(m.Circadian.SleepAt)
	if sleepErr != nil {
		addErr("circadian.sleep_at", fmt.Sprintf("invalid time format %q, expected HH:MM (24-hour)", m.Circadian.SleepAt))
	}

	wakeH, wakeM, wakeErr := ParseTimeHM(m.Circadian.WakeAt)
	if wakeErr != nil {
		addErr("circadian.wake_at", fmt.Sprintf("invalid time format %q, expected HH:MM (24-hour)", m.Circadian.WakeAt))
	}

	if len(m.Circadian.Days) == 0 {
		addErr("circadian.days", "days list cannot be empty")
	} else {
		for _, day := range m.Circadian.Days {
			if !validDays[day] {
				addErr("circadian.days", fmt.Sprintf("invalid day %q, must be one of: Mon, Tue, Wed, Thu, Fri, Sat, Sun", day))
			}
		}
	}

	// Safety buffer check between sleep_at and wake_at
	if sleepErr == nil && wakeErr == nil {
		deltaSec := calculateSleepDurationSeconds(sleepH, sleepM, wakeH, wakeM)
		if deltaSec < MinimumSafetyBufferSeconds {
			addErr("circadian", fmt.Sprintf("circadian safety buffer violation: duration between sleep_at (%s) and wake_at (%s) is %ds, minimum required safety buffer is %ds",
				m.Circadian.SleepAt, m.Circadian.WakeAt, deltaSec, MinimumSafetyBufferSeconds))
		}
	}

	// 4. Teardown
	if !validProviders[m.Teardown.Provider] {
		addErr("teardown.provider", fmt.Sprintf("invalid provider %q, must be one of: docker-compose, podman-compose, command, systemd", m.Teardown.Provider))
	}

	if m.Teardown.TimeoutSeconds <= 0 {
		addErr("teardown.timeout_seconds", "timeout_seconds must be positive")
	}

	if len(m.Teardown.ProfileOrder) == 0 {
		addErr("teardown.profile_order", "profile_order cannot be empty")
	}

	for i, stack := range m.Teardown.ComposeStacks {
		if stack.Path == "" {
			addErr(fmt.Sprintf("teardown.compose_stacks[%d].path", i), "compose stack path cannot be empty")
		}
	}

	// 5. Startup
	if m.Startup.IsEnabled() {
		if !validProviders[m.Startup.Provider] {
			addErr("startup.provider", fmt.Sprintf("invalid provider %q, must be one of: docker-compose, podman-compose, command, systemd", m.Startup.Provider))
		}
		if m.Startup.TimeoutSeconds <= 0 {
			addErr("startup.timeout_seconds", "timeout_seconds must be positive")
		}
		for i, stack := range m.Startup.ComposeStacks {
			if stack.Path == "" {
				addErr(fmt.Sprintf("startup.compose_stacks[%d].path", i), "compose stack path cannot be empty")
			}
		}
	}

	// 6. Hooks
	validateHooks("hooks.pre_teardown", m.Hooks.PreTeardown, addErr)
	validateHooks("hooks.post_teardown", m.Hooks.PostTeardown, addErr)
	validateHooks("hooks.pre_sleep", m.Hooks.PreSleep, addErr)
	validateHooks("hooks.post_wake", m.Hooks.PostWake, addErr)

	// 6. Inhibition
	if m.Inhibition.DefaultDuration != "" {
		dur, err := time.ParseDuration(m.Inhibition.DefaultDuration)
		if err != nil {
			addErr("inhibition.default_duration", fmt.Sprintf("invalid duration %q: %v", m.Inhibition.DefaultDuration, err))
		} else if dur <= 0 {
			addErr("inhibition.default_duration", "duration must be positive")
		}
	}

	if m.Inhibition.DaytimeStart != "" {
		if _, _, err := ParseTimeHM(m.Inhibition.DaytimeStart); err != nil {
			addErr("inhibition.daytime_start", fmt.Sprintf("invalid time format %q, expected HH:MM (24-hour)", m.Inhibition.DaytimeStart))
		}
	}

	if m.Inhibition.DaytimeEnd != "" {
		if _, _, err := ParseTimeHM(m.Inhibition.DaytimeEnd); err != nil {
			addErr("inhibition.daytime_end", fmt.Sprintf("invalid time format %q, expected HH:MM (24-hour)", m.Inhibition.DaytimeEnd))
		}
	}

	if len(errors) > 0 {
		return &ValidationError{Errors: errors}
	}
	return nil
}

func validateHooks(prefix string, hooks []HookConfig, addErr func(field, message string)) {
	for i, h := range hooks {
		if h.Name == "" {
			addErr(fmt.Sprintf("%s[%d].name", prefix, i), "hook name cannot be empty")
		}
		if h.Command == "" {
			addErr(fmt.Sprintf("%s[%d].command", prefix, i), "hook command cannot be empty")
		}
		if h.TimeoutSeconds <= 0 {
			addErr(fmt.Sprintf("%s[%d].timeout_seconds", prefix, i), "hook timeout_seconds must be positive")
		}
	}
}

// ParseTimeHM parses an "HH:MM" 24-hour time string into hours and minutes.
func ParseTimeHM(s string) (int, int, error) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) != 5 || trimmed[2] != ':' {
		return 0, 0, fmt.Errorf("invalid time format %q, expected HH:MM (24-hour)", s)
	}
	h, err1 := strconv.Atoi(trimmed[0:2])
	m, err2 := strconv.Atoi(trimmed[3:5])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time %q: hour must be 00-23 and minute must be 00-59", s)
	}
	return h, m, nil
}

func calculateSleepDurationSeconds(sleepH, sleepM, wakeH, wakeM int) int {
	sleepTotal := sleepH*60 + sleepM
	wakeTotal := wakeH*60 + wakeM

	var deltaMinutes int
	if wakeTotal > sleepTotal {
		deltaMinutes = wakeTotal - sleepTotal
	} else if wakeTotal < sleepTotal {
		deltaMinutes = (24*60 - sleepTotal) + wakeTotal
	} else {
		deltaMinutes = 0
	}

	return deltaMinutes * 60
}
