package rtc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
)

const (
	// MinSafetyBufferSeconds defines the minimum duration between sleep initiation and wake target
	// to prevent ACPI S5 shutdown race conditions.
	MinSafetyBufferSeconds = 180

	// DefaultSysfsPath points to the primary hardware real-time clock sysfs directory.
	DefaultSysfsPath = "/sys/class/rtc/rtc0"

	// DefaultAdjtimePath points to the standard Linux clock configuration file.
	DefaultAdjtimePath = "/etc/adjtime"
)

var (
	ErrAlarmTooSoon         = errors.New("wake alarm target is within the 180-second safety buffer")
	ErrAlarmInPast          = errors.New("wake alarm target is in the past")
	ErrAlarmNotSupported    = errors.New("real-time clock does not support hardware wakealarms")
	ErrPermissionDenied     = errors.New("insufficient privileges to access real-time clock sysfs interface")
	ErrAlarmRetentionFailed = errors.New("hardware rejected wakealarm: retention verification failed")
	ErrInvalidPowerAction   = errors.New("unsupported host power action")
)

type ClockMode string

const (
	ClockModeUTC   ClockMode = "UTC"
	ClockModeLocal ClockMode = "LOCAL"
)

type PowerAction string

const (
	ActionPoweroff  PowerAction = "poweroff"
	ActionSuspend   PowerAction = "suspend"
	ActionHibernate PowerAction = "hibernate"
)

type AlarmStatus struct {
	Enabled   bool          `json:"enabled"`
	Target    time.Time     `json:"target,omitempty"`
	Remaining time.Duration `json:"remaining,omitempty"`
	Device    string        `json:"device"`
	ClockMode ClockMode     `json:"clock_mode"`
}

type Manager struct {
	env       env.HostEnvironment
	sysfsPath string
	adjPath   string
	location  *time.Location
}

type Option func(*Manager)

func WithSysfsPath(path string) Option {
	return func(m *Manager) {
		m.sysfsPath = path
	}
}

func WithAdjtimePath(path string) Option {
	return func(m *Manager) {
		m.adjPath = path
	}
}

func WithLocation(loc *time.Location) Option {
	return func(m *Manager) {
		if loc != nil {
			m.location = loc
		}
	}
}

// NewManager initializes an RTC driver attached to the host environment seam.
func NewManager(environment env.HostEnvironment, opts ...Option) *Manager {
	m := &Manager{
		env:      environment,
		adjPath:  DefaultAdjtimePath,
		location: time.Local,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.sysfsPath == "" {
		if environment != nil {
			if val, ok := environment.LookupEnv("ARKON_RTC_DEVICE"); ok && val != "" {
				m.sysfsPath = val
			}
		}
		if m.sysfsPath == "" {
			if val := os.Getenv("ARKON_RTC_DEVICE"); val != "" {
				m.sysfsPath = val
			} else {
				m.sysfsPath = DefaultSysfsPath
			}
		}
	}
	return m
}

func (m *Manager) wakealarmPath() string {
	if strings.HasSuffix(m.sysfsPath, "/wakealarm") || filepath.Base(m.sysfsPath) == "wakealarm" {
		return m.sysfsPath
	}
	return filepath.Join(m.sysfsPath, "wakealarm")
}

func (m *Manager) loc() *time.Location {
	if m.location != nil {
		return m.location
	}
	return time.Local
}

// DetectClockMode inspects line 3 of /etc/adjtime to determine if the hardware clock ticks in UTC or LOCAL time.
// If /etc/adjtime is absent or line 3 is unreadable, it defaults to UTC following standard Linux convention.
func (m *Manager) DetectClockMode() ClockMode {
	data, err := m.env.ReadFile(m.adjPath)
	if err != nil {
		return ClockModeUTC
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) >= 3 {
		line3 := strings.TrimSpace(lines[2])
		if strings.HasPrefix(strings.ToUpper(line3), "LOCAL") {
			return ClockModeLocal
		}
	}
	return ClockModeUTC
}

// ClearAlarm cancels any pending hardware wakealarm by writing 0 to the sysfs interface.
// Clearing active alarms avoids the Linux kernel -EBUSY condition when reprogramming.
func (m *Manager) ClearAlarm() error {
	wakePath := m.wakealarmPath()

	// Verify device node exists before writing
	if _, err := m.env.ReadFile(wakePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrAlarmNotSupported
		}
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
	}

	if err := m.env.WriteFile(wakePath, []byte("0\n"), 0644); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		if errors.Is(err, os.ErrNotExist) {
			return ErrAlarmNotSupported
		}
		return fmt.Errorf("failed to clear wakealarm: %w", err)
	}
	return nil
}

// ValidateSafetyBuffer checks whether the proposed sleep delta satisfies the 180-second safety window.
func (m *Manager) ValidateSafetyBuffer(deltaSeconds int64) error {
	if deltaSeconds <= 0 {
		return ErrAlarmInPast
	}
	if deltaSeconds < MinSafetyBufferSeconds {
		return fmt.Errorf("%w: requested %d seconds, minimum required is %d seconds", ErrAlarmTooSoon, deltaSeconds, MinSafetyBufferSeconds)
	}
	return nil
}

// ValidateSafetyBufferAt evaluates the proposed wake timestamp against the current environment time.
func (m *Manager) ValidateSafetyBufferAt(target time.Time) error {
	delta := int64(target.Sub(m.env.Now()).Seconds())
	return m.ValidateSafetyBuffer(delta)
}

// SetWakeAlarm schedules a hardware wakealarm relative to the current RTC counter using relative seconds (+<sec>).
// This relative syntax bypasses BIOS UTC versus local time translation during writes.
func (m *Manager) SetWakeAlarm(deltaSeconds int64) error {
	if err := m.ValidateSafetyBuffer(deltaSeconds); err != nil {
		return err
	}

	wakePath := m.wakealarmPath()
	if _, err := m.env.ReadFile(wakePath); errors.Is(err, os.ErrNotExist) {
		return ErrAlarmNotSupported
	}

	// Step 1: Pre-clear active alarms to avoid kernel -EBUSY
	if err := m.ClearAlarm(); err != nil {
		return err
	}

	// Step 2: Write relative seconds (+<seconds>)
	payload := fmt.Sprintf("+%d\n", deltaSeconds)
	if err := m.env.WriteFile(wakePath, []byte(payload), 0644); err != nil {
		if os.IsPermission(err) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("failed to write wakealarm: %w", err)
	}

	// Step 3: Verify hardware retention before returning
	raw, err := m.env.ReadFile(wakePath)
	if err != nil {
		return fmt.Errorf("failed to verify wakealarm: %w", err)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return fmt.Errorf("%w: readback was empty", ErrAlarmRetentionFailed)
	}

	epoch, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: invalid readback value %q", ErrAlarmRetentionFailed, trimmed)
	}
	if epoch <= 0 {
		return fmt.Errorf("%w: readback epoch %d is not positive", ErrAlarmRetentionFailed, epoch)
	}

	return nil
}

// SetWakeAlarmAt computes the delta between now and target wake time, then sets the relative wakealarm.
func (m *Manager) SetWakeAlarmAt(target time.Time) error {
	deltaSeconds := int64(target.Sub(m.env.Now()).Seconds())
	if err := m.ValidateSafetyBuffer(deltaSeconds); err != nil {
		return err
	}
	return m.SetWakeAlarm(deltaSeconds)
}

// GetStatus reads the sysfs wakealarm node and maps the hardware integer epoch to local wall-clock time.
func (m *Manager) GetStatus() (*AlarmStatus, error) {
	wakePath := m.wakealarmPath()
	raw, err := m.env.ReadFile(wakePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrAlarmNotSupported
		}
		return nil, err
	}

	mode := m.DetectClockMode()
	str := strings.TrimSpace(string(raw))
	if str == "" {
		return &AlarmStatus{
			Enabled:   false,
			Device:    m.sysfsPath,
			ClockMode: mode,
		}, nil
	}

	epochSec, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed wakealarm epoch %q: %w", str, err)
	}
	if epochSec <= 0 {
		return &AlarmStatus{
			Enabled:   false,
			Device:    m.sysfsPath,
			ClockMode: mode,
		}, nil
	}

	var target time.Time
	if mode == ClockModeLocal {
		tUTC := time.Unix(epochSec, 0).UTC()
		target = time.Date(tUTC.Year(), tUTC.Month(), tUTC.Day(),
			tUTC.Hour(), tUTC.Minute(), tUTC.Second(), 0, m.loc())
	} else {
		target = time.Unix(epochSec, 0).In(m.loc())
	}

	remaining := target.Sub(m.env.Now())
	return &AlarmStatus{
		Enabled:   true,
		Target:    target,
		Remaining: remaining,
		Device:    m.sysfsPath,
		ClockMode: mode,
	}, nil
}

// ExecutePowerAction triggers the requested ACPI power transition through systemctl.
func (m *Manager) ExecutePowerAction(ctx context.Context, action string) error {
	normalized := strings.ToLower(strings.TrimSpace(action))
	switch normalized {
	case string(ActionPoweroff), string(ActionSuspend), string(ActionHibernate):
	default:
		return fmt.Errorf("%w: %q (supported: poweroff, suspend, hibernate)", ErrInvalidPowerAction, action)
	}

	out, err := m.env.Exec(ctx, "systemctl", normalized)
	if err != nil {
		trimmedOut := strings.TrimSpace(string(out))
		if trimmedOut != "" {
			return fmt.Errorf("failed to execute power action systemctl %s: %w (output: %s)", normalized, err, trimmedOut)
		}
		return fmt.Errorf("failed to execute power action systemctl %s: %w", normalized, err)
	}
	return nil
}
