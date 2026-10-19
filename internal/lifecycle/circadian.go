package lifecycle

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/rtc"
)

// CalculateNextWake computes the next scheduled wake timestamp from circadian policy.
// It searches forward up to 14 days to find the earliest wake window matching active days
// that satisfies the minimum safety buffer (180 seconds).
func CalculateNextWake(now time.Time, circadian manifest.CircadianConfig, tz string) (time.Time, error) {
	loc := time.Local
	if strings.TrimSpace(tz) != "" {
		l, err := time.LoadLocation(strings.TrimSpace(tz))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %q: %w", tz, err)
		}
		loc = l
	}

	wakeH, wakeM, ok := parseTimeHM(circadian.WakeAt)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid circadian.wake_at %q: expected HH:MM (24-hour)", circadian.WakeAt)
	}

	days := circadian.Days
	if len(days) == 0 {
		days = manifest.DefaultDays
	}
	allowedDays := make(map[string]bool, len(days))
	for _, d := range days {
		allowedDays[strings.TrimSpace(d)] = true
	}

	nowInLoc := now.In(loc)

	// Search up to 14 days ahead for the next matching wake window
	for dayOffset := 0; dayOffset <= 14; dayOffset++ {
		date := nowInLoc.AddDate(0, 0, dayOffset)
		cand := time.Date(date.Year(), date.Month(), date.Day(), wakeH, wakeM, 0, 0, loc)

		dayStr := cand.Weekday().String()[:3]
		if !allowedDays[dayStr] {
			continue
		}

		delta := int64(cand.Sub(now).Seconds())
		if delta < rtc.MinSafetyBufferSeconds {
			continue
		}

		return cand, nil
	}

	return time.Time{}, fmt.Errorf("could not compute next wake time within 14 days for wake_at %s on days %v", circadian.WakeAt, days)
}

func parseTimeHM(s string) (int, int, bool) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) != 5 || trimmed[2] != ':' {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(trimmed[0:2])
	m, err2 := strconv.Atoi(trimmed[3:5])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}
