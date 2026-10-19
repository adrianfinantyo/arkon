package status

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/state"
)

func TestCollectFullStatus(t *testing.T) {
	mem := env.NewMemEnv()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}
	fixedTime := time.Date(2026, 10, 18, 17, 42, 0, 0, loc)
	mem.CurrentTime = fixedTime

	// Mock system files
	mem.Files["/etc/hostname"] = []byte("komsos\n")
	mem.Files["/proc/cpuinfo"] = []byte(`processor	: 0
model name	: Intel(R) Pentium(R) Dual  CPU  E5400  @ 2.70GHz
cpu MHz		: 2700.000
`)
	mem.Files["/proc/meminfo"] = []byte(`MemTotal:        4018340 kB
Dirty:                 0 kB
`)
	mem.Files["/etc/adjtime"] = []byte("0.0 0 0.0\n0\nUTC\n")

	// Alarm set for 07:00 WIB (00:00 UTC on 2026-10-19)
	alarmEpoch := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).Unix()
	mem.Files["/sys/class/rtc/rtc0/wakealarm"] = []byte(strconv.FormatInt(alarmEpoch, 10) + "\n")


	// Mock state file with an active lock
	expTime := fixedTime.Add(2 * time.Hour)
	st := state.State{
		Locks: []state.InhibitionLock{
			{
				ID:        "inh-20261018-01",
				Reason:    "Running database backup",
				CreatedBy: "operator",
				ExpiresAt: expTime,
			},
		},
	}
	stBytes, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	mem.Files["/var/lib/arkon/state.json"] = stBytes

	// Mock systemctl is-active for timer
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) == 2 && args[0] == "is-active" && args[1] == "arkon-sleep.timer" {
			return []byte("active\n"), nil
		}
		return nil, nil
	}

	m := &manifest.Manifest{
		Version: "1",
		Host: manifest.HostConfig{
			Timezone:    "Asia/Jakarta",
			RTCDevice:   "/sys/class/rtc/rtc0",
			PowerAction: "poweroff",
		},
		Circadian: manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
		},
		Teardown: manifest.TeardownConfig{
			Provider:     "docker-compose",
			ProfileOrder: []string{"apps", "infra"},
			ComposeStacks: []manifest.ComposeStack{
				{Path: "/opt/stacks/apps"},
			},
		},
	}

	data, err := Collect(context.Background(), mem, m, "/var/lib/arkon")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if data.HostPowerState != "ACPI S0 (Operational Active)" {
		t.Errorf("expected host power state ACPI S0, got %q", data.HostPowerState)
	}
	if data.Hostname != "komsos" {
		t.Errorf("expected hostname komsos, got %q", data.Hostname)
	}
	if !strings.Contains(data.CPUArch, "Intel(R) Pentium(R) Dual") && !strings.Contains(data.CPUArch, "Intel") {
		t.Errorf("expected CPU architecture to contain Intel info, got %q", data.CPUArch)
	}
	if !strings.Contains(data.Timezone, "Asia/Jakarta") || !strings.Contains(data.Timezone, "WIB") {
		t.Errorf("expected timezone Asia/Jakarta (WIB), got %q", data.Timezone)
	}

	// RTC wakealarm checks
	if data.RTCWakeAlarm == nil || !data.RTCWakeAlarm.Enabled {
		t.Fatalf("expected enabled RTC wakealarm, got %+v", data.RTCWakeAlarm)
	}
	if data.RTCWakeAlarm.Device != "/sys/class/rtc/rtc0" {
		t.Errorf("expected device /sys/class/rtc/rtc0, got %q", data.RTCWakeAlarm.Device)
	}

	// Circadian timer checks
	if !data.CircadianTimer.Active {
		t.Errorf("expected circadian timer active: true")
	}
	if data.CircadianTimer.SleepAt != "23:00" || data.CircadianTimer.WakeAt != "07:00" {
		t.Errorf("unexpected circadian sleep/wake times: %s / %s", data.CircadianTimer.SleepAt, data.CircadianTimer.WakeAt)
	}
	if !strings.Contains(data.CircadianTimer.NextAction, "Sleep") {
		t.Errorf("expected NextAction to be Sleep during active daytime, got %q", data.CircadianTimer.NextAction)
	}
	if !strings.Contains(data.CircadianTimer.NextTime, "23:00:00 WIB") {
		t.Errorf("expected NextTime to mention 23:00:00 WIB, got %q", data.CircadianTimer.NextTime)
	}

	// Inhibition locks checks
	if len(data.InhibitionLocks) != 1 {
		t.Fatalf("expected 1 active inhibition lock, got %d", len(data.InhibitionLocks))
	}
	if data.InhibitionLocks[0].ID != "inh-20261018-01" {
		t.Errorf("expected lock id inh-20261018-01, got %q", data.InhibitionLocks[0].ID)
	}
	if data.InhibitionLocks[0].Remaining == "" {
		t.Errorf("expected populated Remaining for lock")
	}

	// Teardown checks
	if data.Teardown.Provider != "docker-compose" {
		t.Errorf("expected provider docker-compose, got %q", data.Teardown.Provider)
	}
	if data.Teardown.StackCount != 1 {
		t.Errorf("expected stack count 1, got %d", data.Teardown.StackCount)
	}
	if len(data.Teardown.ProfileOrder) != 2 || data.Teardown.ProfileOrder[0] != "apps" {
		t.Errorf("unexpected profile order: %v", data.Teardown.ProfileOrder)
	}
}

func TestCollectUnprivilegedExecution(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000 // Non-root unprivileged operator

	// No state file existing yet
	data, err := Collect(context.Background(), mem, nil, "/var/lib/arkon")
	if err != nil {
		t.Fatalf("Collect failed for unprivileged user: %v", err)
	}

	if data.HostPowerState != "ACPI S0 (Operational Active)" {
		t.Errorf("expected ACPI S0, got %q", data.HostPowerState)
	}
	if len(data.InhibitionLocks) != 0 {
		t.Errorf("expected 0 locks when state file is absent, got %d", len(data.InhibitionLocks))
	}
	if data.RTCWakeAlarm == nil || data.RTCWakeAlarm.Enabled {
		t.Errorf("expected disabled RTC alarm when sysfs file absent")
	}
}

func TestCollectDisabledRTCWakealarm(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/sys/class/rtc/rtc0/wakealarm"] = []byte("0\n")
	mem.Files["/etc/adjtime"] = []byte("0.0 0 0.0\n0\nUTC\n")

	data, err := Collect(context.Background(), mem, nil, "/var/lib/arkon")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if data.RTCWakeAlarm == nil {
		t.Fatal("expected non-nil RTCWakeAlarm")
	}
	if data.RTCWakeAlarm.Enabled {
		t.Errorf("expected RTCWakeAlarm.Enabled to be false when sysfs has 0")
	}
}

func TestCollectClockModeLocal(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/sys/class/rtc/rtc0/wakealarm"] = []byte("1792368000\n")
	mem.Files["/etc/adjtime"] = []byte("0.0 0 0.0\n0\nLOCAL\n")

	data, err := Collect(context.Background(), mem, nil, "/var/lib/arkon")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if data.RTCWakeAlarm == nil {
		t.Fatal("expected non-nil RTCWakeAlarm")
	}
	if data.RTCWakeAlarm.ClockMode != "LOCAL" {
		t.Errorf("expected ClockMode LOCAL, got %q", data.RTCWakeAlarm.ClockMode)
	}
}

func TestCircadianOvernightSchedule(t *testing.T) {
	mem := env.NewMemEnv()
	// Set time to 21:00 UTC (awake during overnight schedule wake_at: 20:00, sleep_at: 04:00)
	now := time.Date(2026, 10, 18, 21, 0, 0, 0, time.UTC)
	mem.CurrentTime = now

	m := &manifest.Manifest{
		Version: "1",
		Host: manifest.HostConfig{
			Timezone: "UTC",
		},
		Circadian: manifest.CircadianConfig{
			WakeAt:  "20:00",
			SleepAt: "04:00",
		},
	}

	data, err := Collect(context.Background(), mem, m, "/var/lib/arkon")
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if !strings.Contains(data.CircadianTimer.NextAction, "Sleep") {
		t.Errorf("expected NextAction Sleep at 21:00 for overnight schedule, got %q", data.CircadianTimer.NextAction)
	}
	if !strings.Contains(data.CircadianTimer.NextTime, "04:00:00 UTC") {
		t.Errorf("expected NextTime to mention 04:00:00 UTC, got %q", data.CircadianTimer.NextTime)
	}
}

func TestFormatStatusTextLayout(t *testing.T) {
	data := &StatusData{
		HostPowerState: "ACPI S0 (Operational Active)",
		Hostname:       "komsos",
		CPUArch:        "Intel Pentium Dual-Core E5400 (x86_64)",
		Timezone:       "Asia/Jakarta (WIB, UTC+07:00)",
		RTCWakeAlarm: &RTCStatus{
			Enabled:       true,
			AlarmTimeUTC:  "2026-10-19T00:00:00Z",
			AlarmTimeHost: "2026-10-19 07:00:00 WIB",
			SecondsUntil:  26280,
			Device:        "/sys/class/rtc/rtc0",
			ClockMode:     "UTC",
			KernelDirty:   "0 kB",
		},
		CircadianTimer: CircadianStatus{
			SleepAt:    "23:00",
			WakeAt:     "07:00",
			NextAction: "Sleep (ACPI S5 Soft-Off)",
			NextTime:   "Today at 23:00:00 WIB (in 5h 18m)",
			TimerUnit:  "arkon-sleep.timer",
			Active:     true,
		},
		InhibitionLocks: []state.InhibitionLock{
			{
				ID:        "inh-20261018-01",
				Reason:    "Running database backup",
				CreatedBy: "operator",
				Remaining: "1h 14m",
			},
		},
		Teardown: TeardownStatus{
			Provider:     "docker-compose",
			ProfileOrder: []string{"apps", "infra"},
			StackCount:   1,
			Stacks:       []string{"/opt/stacks/apps"},
		},
	}

	output := FormatStatus(data, false)

	// Check sections and bullets
	expectedHeadings := []string{
		"● ARKON HOST STATUS",
		"● Circadian Schedule",
		"● Hardware RTC Wakealarm",
		"● Workload Teardown Pipeline",
		"● Maintenance Locks",
	}

	for _, heading := range expectedHeadings {
		if !strings.Contains(output, heading) {
			t.Errorf("expected formatted output to contain heading %q", heading)
		}
	}

	// Verify two-space indent on key lines
	expectedIndentedLines := []string{
		"  Host:",
		"  Timezone:",
		"  Schedule:",
		"  Next Action:",
		"  Device:",
		"  Target:",
		"  Provider:",
		"  Progression:",
	}

	for _, line := range expectedIndentedLines {
		if !strings.Contains(output, line) {
			t.Errorf("expected formatted output to contain two-space indented line %q", line)
		}
	}

	// Verify content values
	if !strings.Contains(output, "komsos") {
		t.Errorf("missing hostname komsos")
	}
	if !strings.Contains(output, "Intel Pentium Dual-Core E5400") {
		t.Errorf("missing CPU arch")
	}
	if !strings.Contains(output, "inh-20261018-01") {
		t.Errorf("missing lock ID")
	}
	if !strings.Contains(output, "Running database backup") {
		t.Errorf("missing lock reason")
	}

	// Test no-color output contains no ANSI escape codes
	noColorOut := FormatStatus(data, true)
	if strings.Contains(noColorOut, "\033[") {
		t.Errorf("expected no-color output to have no ANSI escapes, found some: %s", noColorOut)
	}
}

func TestStatusJSONSerialization(t *testing.T) {
	data := &StatusData{
		HostPowerState: "ACPI S0 (Operational Active)",
		Hostname:       "komsos",
		CPUArch:        "x86_64",
		Timezone:       "UTC",
		RTCWakeAlarm: &RTCStatus{
			Enabled: false,
			Device:  "/sys/class/rtc/rtc0",
		},
		CircadianTimer: CircadianStatus{
			TimerUnit: "arkon-sleep.timer",
			Active:    false,
		},
		InhibitionLocks: []state.InhibitionLock{},
		Teardown: TeardownStatus{
			Provider:   "docker-compose",
			StackCount: 0,
		},
	}

	type envelope struct {
		Success bool        `json:"success"`
		Data    *StatusData `json:"data"`
		Error   any         `json:"error"`
	}

	envObj := envelope{
		Success: true,
		Data:    data,
		Error:   nil,
	}

	marshaled, err := json.Marshal(envObj)
	if err != nil {
		t.Fatalf("failed to marshal envelope with StatusData: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(marshaled, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if result["success"] != true {
		t.Errorf("expected success: true")
	}
	dataMap, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data map in JSON, got %T", result["data"])
	}
	if dataMap["hostname"] != "komsos" {
		t.Errorf("expected hostname komsos, got %v", dataMap["hostname"])
	}
}
