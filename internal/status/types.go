package status

import (
	"github.com/adrianfinantyo/arkon/internal/state"
)

// StatusData represents the complete diagnostic data model for Arkon host status.
type StatusData struct {
	Configured      bool                   `json:"configured"`
	ActiveManifest  string                 `json:"active_manifest,omitempty"`
	ManifestError   string                 `json:"manifest_error,omitempty"`
	DriftNotice     string                 `json:"drift_notice,omitempty"`
	HostPowerState  string                 `json:"host_power_state"`
	Hostname        string                 `json:"hostname"`
	CPUArch         string                 `json:"cpu_arch"`
	Timezone        string                 `json:"timezone"`
	RTCWakeAlarm    *RTCStatus             `json:"rtc_wake_alarm"`
	CircadianTimer  CircadianStatus        `json:"circadian_timer"`
	InhibitionLocks []state.InhibitionLock `json:"inhibition_locks"`
	Teardown        TeardownStatus         `json:"teardown"`
	Startup         StartupStatus          `json:"startup"`
	Hooks           HooksStatus            `json:"hooks"`
}

// RTCStatus represents the hardware real-time clock wakealarm state.
type RTCStatus struct {
	Enabled       bool   `json:"enabled"`
	AlarmTimeUTC  string `json:"alarm_time_utc,omitempty"`
	AlarmTimeHost string `json:"alarm_time_host,omitempty"`
	SecondsUntil  int64  `json:"seconds_until,omitempty"`
	Device        string `json:"device"`
	ClockMode     string `json:"clock_mode,omitempty"`
	KernelDirty   string `json:"kernel_dirty,omitempty"`
}

// CircadianStatus represents the daily sleep and wake policy.
type CircadianStatus struct {
	Configured bool   `json:"configured"`
	SleepAt    string `json:"sleep_at"`
	WakeAt     string `json:"wake_at"`
	NextAction string `json:"next_action"`
	NextTime   string `json:"next_time"`
	TimerUnit  string `json:"timer_unit"`
	Active     bool   `json:"active"`
}

// TeardownStatus represents the workload shutdown configuration.
type TeardownStatus struct {
	Provider     string   `json:"provider"`
	ProfileOrder []string `json:"profile_order"`
	StackCount   int      `json:"stack_count"`
	Stacks       []string `json:"stacks,omitempty"`
}

// StartupStatus represents the workload startup configuration upon boot.
type StartupStatus struct {
	Provider    string   `json:"provider"`
	Profiles    []string `json:"profiles"`
	StackCount  int      `json:"stack_count"`
	Stacks      []string `json:"stacks,omitempty"`
	BootUnit    string   `json:"boot_unit"`
	BootEnabled bool     `json:"boot_enabled"`
}

// HooksStatus represents configured lifecycle hook names by phase.
type HooksStatus struct {
	PreTeardown  []string `json:"pre_teardown"`
	PostTeardown []string `json:"post_teardown"`
	PreSleep     []string `json:"pre_sleep"`
	PostWake     []string `json:"post_wake"`
}
