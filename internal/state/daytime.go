package state

import (
	"fmt"
	"time"

	"github.com/adrianfinantyo/arkon/internal/manifest"
)

// DaytimeConfig contains operational active window settings.
type DaytimeConfig struct {
	WakeAt            string `yaml:"wake_at" json:"wake_at"`
	SleepAt           string `yaml:"sleep_at" json:"sleep_at"`
	DaytimeStart      string `yaml:"daytime_start,omitempty" json:"daytime_start,omitempty"`
	DaytimeEnd        string `yaml:"daytime_end,omitempty" json:"daytime_end,omitempty"`
	DaytimeProtection bool   `yaml:"daytime_protection" json:"daytime_protection"`
}

// SleepCheckResult contains the evaluation of whether a sleep transition is permitted.
type SleepCheckResult struct {
	Inhibited bool
	Code      string
	Reason    string
	Lock      *InhibitionLock
}

// parseTimeMinutes converts an "HH:MM" 24-hour string into total minutes since midnight.
func parseTimeMinutes(s string) (int, error) {
	h, m, err := manifest.ParseTimeHM(s)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// IsDaytimeOperational reports whether the provided time falls within the daytime operational window.
// The operational window spans from wake_at until sleep_at.
func IsDaytimeOperational(t time.Time, wakeAt, sleepAt string) (bool, error) {
	wakeMinutes, err := parseTimeMinutes(wakeAt)
	if err != nil {
		return false, err
	}

	sleepMinutes, err := parseTimeMinutes(sleepAt)
	if err != nil {
		return false, err
	}

	currentMinutes := t.Hour()*60 + t.Minute()

	if wakeMinutes == sleepMinutes {
		return false, nil
	}

	// Normal schedule: wake at morning (e.g. 07:00), sleep at night (e.g. 23:00)
	if wakeMinutes < sleepMinutes {
		return currentMinutes >= wakeMinutes && currentMinutes < sleepMinutes, nil
	}

	// Overnight schedule: wake in evening (e.g. 20:00), sleep in morning (e.g. 04:00)
	return currentMinutes >= wakeMinutes || currentMinutes < sleepMinutes, nil
}

// EvaluateSleepAllowed evaluates whether a host sleep transition may proceed.
// If force is true, daytime protection and active locks are bypassed.
func EvaluateSleepAllowed(now time.Time, daytime DaytimeConfig, activeLocks []InhibitionLock, force bool) SleepCheckResult {
	if force {
		return SleepCheckResult{Inhibited: false}
	}

	start := daytime.DaytimeStart
	if start == "" {
		start = daytime.WakeAt
	}
	end := daytime.DaytimeEnd
	if end == "" {
		end = daytime.SleepAt
	}

	if daytime.DaytimeProtection && start != "" && end != "" {
		isDaytime, err := IsDaytimeOperational(now, start, end)
		if err == nil && isDaytime {
			return SleepCheckResult{
				Inhibited: true,
				Code:      "ERR_DAYTIME_PROTECTION",
				Reason: fmt.Sprintf(
					"Refusing sleep: current time %s is within daytime operational window (wake_at: %s, sleep_at: %s). Pass --force to override.",
					now.Format("15:04"),
					start,
					end,
				),
			}
		}
	}

	if len(activeLocks) > 0 {
		first := activeLocks[0]
		return SleepCheckResult{
			Inhibited: true,
			Code:      "ERR_INHIBITED",
			Reason: fmt.Sprintf(
				"Sleep transition inhibited by active lock '%s' (%q). Release with 'arkon inhibit release %s' or override with '--force'.",
				first.ID,
				first.Reason,
				first.ID,
			),
			Lock: &first,
		}
	}

	return SleepCheckResult{Inhibited: false}
}
