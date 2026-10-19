package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/state"
)

// Collect inspects the host environment, manifest, and runtime state to assemble StatusData.
func Collect(ctx context.Context, hostEnv env.HostEnvironment, m *manifest.Manifest, stateDir string) (*StatusData, error) {
	if hostEnv == nil {
		hostEnv = env.NewRealEnv()
	}
	if stateDir == "" {
		stateDir = state.DefaultStateDir
	}

	data := &StatusData{
		HostPowerState:  "ACPI S0 (Operational Active)",
		InhibitionLocks: []state.InhibitionLock{},
		Teardown: TeardownStatus{
			Provider:     "docker-compose",
			ProfileOrder: []string{},
			Stacks:       []string{},
		},
	}

	data.Hostname = collectHostname(hostEnv)
	data.CPUArch = collectCPUArch(hostEnv)

	loc, tzString := collectTimezone(hostEnv, m)
	data.Timezone = tzString

	data.RTCWakeAlarm = collectRTC(hostEnv, m, loc)
	data.CircadianTimer = collectCircadian(ctx, hostEnv, m, loc)
	data.InhibitionLocks = collectLocks(hostEnv, stateDir)

	if m != nil {
		data.Teardown = collectTeardown(m)
	}

	return data, nil
}

func collectHostname(hostEnv env.HostEnvironment) string {
	if content, err := hostEnv.ReadFile("/etc/hostname"); err == nil {
		trimmed := strings.TrimSpace(string(content))
		if trimmed != "" {
			return trimmed
		}
	}
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}
	return "localhost"
}

func collectCPUArch(hostEnv env.HostEnvironment) string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	content, err := hostEnv.ReadFile("/proc/cpuinfo")
	if err != nil {
		return arch
	}

	lines := strings.Split(string(content), "\n")
	var model string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "model name") || strings.HasPrefix(trimmed, "Model") || strings.HasPrefix(trimmed, "Processor") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				model = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	if model != "" {
		return fmt.Sprintf("%s (%s)", model, arch)
	}
	return arch
}

func collectTimezone(hostEnv env.HostEnvironment, m *manifest.Manifest) (*time.Location, string) {
	tzName := ""
	if m != nil && m.Host.Timezone != "" {
		tzName = strings.TrimSpace(m.Host.Timezone)
	}
	if tzName == "" {
		if content, err := hostEnv.ReadFile("/etc/timezone"); err == nil {
			tzName = strings.TrimSpace(string(content))
		}
	}
	if tzName == "" {
		tzName = "UTC"
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.UTC
		return loc, tzName
	}

	now := hostEnv.Now().In(loc)
	zoneName, offsetSec := now.Zone()

	offsetHours := offsetSec / 3600
	offsetMins := (offsetSec % 3600) / 60
	if offsetMins < 0 {
		offsetMins = -offsetMins
	}
	offsetStr := fmt.Sprintf("UTC%+03d:%02d", offsetHours, offsetMins)

	if tzName == "UTC" && zoneName == "UTC" {
		return loc, "UTC"
	}

	return loc, fmt.Sprintf("%s (%s, %s)", tzName, zoneName, offsetStr)
}

func collectRTC(hostEnv env.HostEnvironment, m *manifest.Manifest, loc *time.Location) *RTCStatus {
	device := "/sys/class/rtc/rtc0"
	if m != nil && m.Host.RTCDevice != "" {
		device = m.Host.RTCDevice
	}

	status := &RTCStatus{
		Device:    device,
		ClockMode: "UTC",
	}

	if adjContent, err := hostEnv.ReadFile("/etc/adjtime"); err == nil {
		lines := strings.Split(string(adjContent), "\n")
		if len(lines) >= 3 {
			line3 := strings.TrimSpace(lines[2])
			if strings.EqualFold(line3, "LOCAL") {
				status.ClockMode = "LOCAL"
			}
		}
	}

	if memContent, err := hostEnv.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(memContent), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Dirty:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					status.KernelDirty = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}

	wakePath := device
	if !strings.HasSuffix(wakePath, "/wakealarm") {
		wakePath = filepath.Join(device, "wakealarm")
	}

	wakeContent, err := hostEnv.ReadFile(wakePath)
	if err != nil {
		return status
	}

	trimmed := strings.TrimSpace(string(wakeContent))
	epoch, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || epoch <= 0 {
		return status
	}

	status.Enabled = true

	var targetUTC time.Time
	var targetHost time.Time

	if status.ClockMode == "LOCAL" {
		tUTC := time.Unix(epoch, 0).UTC()
		targetHost = time.Date(tUTC.Year(), tUTC.Month(), tUTC.Day(),
			tUTC.Hour(), tUTC.Minute(), tUTC.Second(), 0, loc)
		targetUTC = targetHost.UTC()
	} else {
		targetUTC = time.Unix(epoch, 0).UTC()
		targetHost = targetUTC.In(loc)
	}

	status.AlarmTimeUTC = targetUTC.Format(time.RFC3339)
	status.AlarmTimeHost = targetHost.Format("2006-01-02 15:04:05 MST")
	status.SecondsUntil = int64(targetUTC.Sub(hostEnv.Now().UTC()).Seconds())

	return status
}

func collectCircadian(ctx context.Context, hostEnv env.HostEnvironment, m *manifest.Manifest, loc *time.Location) CircadianStatus {
	st := CircadianStatus{
		TimerUnit:  "arkon-sleep.timer",
		NextAction: "None configured",
	}

	out, err := hostEnv.Exec(ctx, "systemctl", "is-active", "arkon-sleep.timer")
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		st.Active = true
	}

	if m == nil || (m.Circadian.SleepAt == "" && m.Circadian.WakeAt == "") {
		return st
	}

	st.SleepAt = m.Circadian.SleepAt
	st.WakeAt = m.Circadian.WakeAt

	powerAction := "poweroff"
	if m.Host.PowerAction != "" {
		powerAction = m.Host.PowerAction
	}

	action, nextTime := evaluateCircadianNext(hostEnv.Now(), loc, st.SleepAt, st.WakeAt, powerAction, m.Circadian.Days)
	st.NextAction = action
	st.NextTime = nextTime

	return st
}

func evaluateCircadianNext(now time.Time, loc *time.Location, sleepAt, wakeAt, powerAction string, days []string) (string, string) {
	if sleepAt == "" || wakeAt == "" {
		return "None configured", ""
	}

	sleepHour, sleepMin, err1 := manifest.ParseTimeHM(sleepAt)
	wakeHour, wakeMin, err2 := manifest.ParseTimeHM(wakeAt)
	if err1 != nil || err2 != nil {
		return "Invalid schedule", ""
	}

	nowInLoc := now.In(loc)
	isDaytime, err := state.IsDaytimeOperational(nowInLoc, wakeAt, sleepAt)
	if err != nil {
		return "Invalid schedule", ""
	}

	activeDays := days
	if len(activeDays) == 0 {
		activeDays = manifest.DefaultDays
	}
	allowedDays := make(map[string]bool, len(activeDays))
	for _, d := range activeDays {
		allowedDays[strings.TrimSpace(d)] = true
	}

	var action string
	var target time.Time

	if isDaytime {
		switch strings.ToLower(powerAction) {
		case "suspend":
			action = "Sleep (ACPI S3 Suspend)"
		case "hibernate":
			action = "Sleep (ACPI S4 Hibernate)"
		default:
			action = "Sleep (ACPI S5 Soft-Off)"
		}
		target = nextActiveDayTime(nowInLoc, loc, sleepHour, sleepMin, allowedDays)
	} else {
		action = "Wake (Hardware RTC)"
		target = nextActiveDayTime(nowInLoc, loc, wakeHour, wakeMin, allowedDays)
	}

	nowDay := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, loc)
	targetDay := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, loc)
	daysDiff := int(targetDay.Sub(nowDay).Hours() / 24)

	var prefix string
	if daysDiff == 0 {
		prefix = "Today at "
	} else if daysDiff == 1 {
		prefix = "Tomorrow at "
	} else {
		prefix = target.Format("2006-01-02 at ")
	}

	timeStr := target.Format("15:04:05 MST")
	remaining := state.FormatRemaining(target.Sub(nowInLoc))
	nextTimeStr := fmt.Sprintf("%s%s (in %s)", prefix, timeStr, remaining)

	return action, nextTimeStr
}

func nextActiveDayTime(nowInLoc time.Time, loc *time.Location, hour, minute int, allowedDays map[string]bool) time.Time {
	for dayOffset := 0; dayOffset <= 14; dayOffset++ {
		date := nowInLoc.AddDate(0, 0, dayOffset)
		cand := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, loc)
		if !cand.After(nowInLoc) {
			continue
		}
		dayStr := cand.Weekday().String()[:3]
		if !allowedDays[dayStr] {
			continue
		}
		return cand
	}
	target := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), hour, minute, 0, 0, loc)
	if !target.After(nowInLoc) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}

func collectLocks(hostEnv env.HostEnvironment, stateDir string) []state.InhibitionLock {
	stateFilePath := filepath.Join(stateDir, "state.json")
	content, err := hostEnv.ReadFile(stateFilePath)
	if err != nil {
		return []state.InhibitionLock{}
	}

	var st state.State
	if err := json.Unmarshal(content, &st); err != nil {
		return []state.InhibitionLock{}
	}

	now := hostEnv.Now()
	active, _ := state.PruneExpiredLocks(st.Locks, now)
	for i := range active {
		active[i].Remaining = state.FormatRemaining(active[i].ExpiresAt.Sub(now))
	}

	if active == nil {
		return []state.InhibitionLock{}
	}
	return active
}

func collectTeardown(m *manifest.Manifest) TeardownStatus {
	provider := m.Teardown.Provider
	if provider == "" {
		provider = "docker-compose"
	}
	profileOrder := m.Teardown.ProfileOrder
	if profileOrder == nil {
		profileOrder = []string{}
	}
	stacks := make([]string, 0, len(m.Teardown.ComposeStacks))
	for _, s := range m.Teardown.ComposeStacks {
		if s.Path != "" {
			stacks = append(stacks, s.Path)
		}
	}
	return TeardownStatus{
		Provider:     provider,
		ProfileOrder: profileOrder,
		StackCount:   len(m.Teardown.ComposeStacks),
		Stacks:       stacks,
	}
}
