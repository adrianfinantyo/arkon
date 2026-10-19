package rtc_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/rtc"
)

// setupRTCEnv builds a MemEnv populated with a simulated RTC sysfs and adjtime file.
func setupRTCEnv(t *testing.T, initialAlarm string, adjtimeContent string) (*env.MemEnv, []string) {
	t.Helper()
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

	var writeLog []string
	wakePath := "/sys/class/rtc/rtc0/wakealarm"
	adjPath := "/etc/adjtime"

	mem.Files[wakePath] = []byte(initialAlarm)
	if adjtimeContent != "" {
		mem.Files[adjPath] = []byte(adjtimeContent)
	}

	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
		writeLog = append(writeLog, string(data))
		str := strings.TrimSpace(string(data))

		if path == wakePath {
			if str == "0" {
				mem.Files[path] = []byte("")
				return nil
			}
			if strings.HasPrefix(str, "+") {
				secStr := strings.TrimPrefix(str, "+")
				sec, err := strconv.ParseInt(secStr, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid relative format: %w", err)
				}
				targetEpoch := mem.CurrentTime.Unix() + sec
				mem.Files[path] = []byte(fmt.Sprintf("%d\n", targetEpoch))
				return nil
			}
			// Absolute epoch format
			epoch, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid epoch format: %w", err)
			}
			mem.Files[path] = []byte(fmt.Sprintf("%d\n", epoch))
			return nil
		}

		mem.Files[path] = data
		return nil
	}

	return mem, writeLog
}

func TestDetectClockMode(t *testing.T) {
	tests := []struct {
		name     string
		adjtime  string
		expected rtc.ClockMode
	}{
		{
			name:     "missing adjtime defaults to UTC",
			adjtime:  "",
			expected: rtc.ClockModeUTC,
		},
		{
			name:     "adjtime with UTC",
			adjtime:  "0.000000 0 0.000000\n0\nUTC\n",
			expected: rtc.ClockModeUTC,
		},
		{
			name:     "adjtime with LOCAL",
			adjtime:  "0.000000 0 0.000000\n0\nLOCAL\n",
			expected: rtc.ClockModeLocal,
		},
		{
			name:     "adjtime with lowercase local",
			adjtime:  "0.000000 0 0.000000\n0\nlocal\n",
			expected: rtc.ClockModeLocal,
		},
		{
			name:     "truncated adjtime defaults to UTC",
			adjtime:  "0.000000 0 0.000000\n",
			expected: rtc.ClockModeUTC,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mem := env.NewMemEnv()
			if tc.adjtime != "" {
				mem.Files["/etc/adjtime"] = []byte(tc.adjtime)
			}

			mgr := rtc.NewManager(mem)
			mode := mgr.DetectClockMode()
			if mode != tc.expected {
				t.Fatalf("expected clock mode %q, got %q", tc.expected, mode)
			}
		})
	}
}

func TestClearAlarm(t *testing.T) {
	t.Run("writes zero to wakealarm when alarm exists", func(t *testing.T) {
		mem, _ := setupRTCEnv(t, "1729340000\n", "")
		mgr := rtc.NewManager(mem)

		if err := mgr.ClearAlarm(); err != nil {
			t.Fatalf("ClearAlarm failed: %v", err)
		}

		readBack, err := mem.ReadFile("/sys/class/rtc/rtc0/wakealarm")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}
		if strings.TrimSpace(string(readBack)) != "" {
			t.Fatalf("expected empty alarm after clear, got %q", string(readBack))
		}
	})

	t.Run("returns ErrAlarmNotSupported when wakealarm path does not exist", func(t *testing.T) {
		mem := env.NewMemEnv()
		mgr := rtc.NewManager(mem)

		err := mgr.ClearAlarm()
		if !errors.Is(err, rtc.ErrAlarmNotSupported) {
			t.Fatalf("expected ErrAlarmNotSupported, got %v", err)
		}
	})

	t.Run("returns ErrPermissionDenied on permission failure", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/sys/class/rtc/rtc0/wakealarm"] = []byte("1729340000\n")
		mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
			return os.ErrPermission
		}

		mgr := rtc.NewManager(mem)
		err := mgr.ClearAlarm()
		if !errors.Is(err, rtc.ErrPermissionDenied) {
			t.Fatalf("expected ErrPermissionDenied, got %v", err)
		}
	})
}

func TestSetWakeAlarmPreClearAndRelativeWrite(t *testing.T) {
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

	wakePath := "/sys/class/rtc/rtc0/wakealarm"
	mem.Files[wakePath] = []byte("1729340000\n")

	var writeHistory []string
	alarmActive := true

	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
		str := string(data)
		writeHistory = append(writeHistory, str)

		if path == wakePath {
			trimmed := strings.TrimSpace(str)
			if trimmed == "0" {
				alarmActive = false
				mem.Files[path] = []byte("")
				return nil
			}

			// Simulating Linux kernel drivers/rtc/sysfs.c:
			// If an alarm is already enabled and new alarm is written without pre-clearing,
			// the kernel returns -EBUSY!
			if alarmActive {
				return syscall.EBUSY
			}

			if strings.HasPrefix(trimmed, "+") {
				sec, err := strconv.ParseInt(strings.TrimPrefix(trimmed, "+"), 10, 64)
				if err != nil {
					return err
				}
				targetEpoch := mem.CurrentTime.Unix() + sec
				alarmActive = true
				mem.Files[path] = []byte(fmt.Sprintf("%d\n", targetEpoch))
				return nil
			}

			return errors.New("expected relative syntax")
		}
		mem.Files[path] = data
		return nil
	}

	mgr := rtc.NewManager(mem)

	// Set alarm 3600 seconds in future
	if err := mgr.SetWakeAlarm(3600); err != nil {
		t.Fatalf("SetWakeAlarm failed: %v", err)
	}

	// Verify write sequence: pre-clear write must precede relative write
	if len(writeHistory) < 2 {
		t.Fatalf("expected at least 2 writes, got %d (%v)", len(writeHistory), writeHistory)
	}
	if strings.TrimSpace(writeHistory[0]) != "0" {
		t.Fatalf("expected first write to be pre-clear 0, got %q", writeHistory[0])
	}
	if writeHistory[1] != "+3600\n" {
		t.Fatalf("expected second write to be +3600\\n, got %q", writeHistory[1])
	}

	// Verify readback contains updated integer epoch
	readBack, err := mem.ReadFile(wakePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	expectedEpoch := fmt.Sprintf("%d\n", mem.CurrentTime.Unix()+3600)
	if string(readBack) != expectedEpoch {
		t.Fatalf("expected readback %q, got %q", expectedEpoch, string(readBack))
	}
}

func TestSetWakeAlarmVerificationRetention(t *testing.T) {
	t.Run("fails when hardware rejects alarm by returning empty string", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.CurrentTime = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
		wakePath := "/sys/class/rtc/rtc0/wakealarm"
		mem.Files[wakePath] = []byte("")

		mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
			// Simulate hardware ignoring the write, leaving wakealarm empty
			mem.Files[path] = []byte("")
			return nil
		}

		mgr := rtc.NewManager(mem)
		err := mgr.SetWakeAlarm(300)
		if err == nil {
			t.Fatal("expected error when hardware rejects alarm with empty readback")
		}
		if !errors.Is(err, rtc.ErrAlarmRetentionFailed) {
			t.Fatalf("expected ErrAlarmRetentionFailed, got %v", err)
		}
	})

	t.Run("fails when readback contains non-positive integer", func(t *testing.T) {
		mem := env.NewMemEnv()
		wakePath := "/sys/class/rtc/rtc0/wakealarm"
		mem.Files[wakePath] = []byte("")

		mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
			if strings.TrimSpace(string(data)) == "0" {
				mem.Files[path] = []byte("")
				return nil
			}
			mem.Files[path] = []byte("0\n")
			return nil
		}

		mgr := rtc.NewManager(mem)
		err := mgr.SetWakeAlarm(300)
		if err == nil {
			t.Fatal("expected error when readback is 0")
		}
		if !errors.Is(err, rtc.ErrAlarmRetentionFailed) {
			t.Fatalf("expected ErrAlarmRetentionFailed, got %v", err)
		}
	})
}

func TestSafetyBufferEnforcement(t *testing.T) {
	mem, _ := setupRTCEnv(t, "", "")
	mgr := rtc.NewManager(mem)

	cases := []struct {
		deltaSeconds int64
		expectErr    error
	}{
		{deltaSeconds: -10, expectErr: rtc.ErrAlarmInPast},
		{deltaSeconds: 0, expectErr: rtc.ErrAlarmInPast},
		{deltaSeconds: 1, expectErr: rtc.ErrAlarmTooSoon},
		{deltaSeconds: 60, expectErr: rtc.ErrAlarmTooSoon},
		{deltaSeconds: 179, expectErr: rtc.ErrAlarmTooSoon},
		{deltaSeconds: 180, expectErr: nil},
		{deltaSeconds: 300, expectErr: nil},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("delta_%d", tc.deltaSeconds), func(t *testing.T) {
			err := mgr.ValidateSafetyBuffer(tc.deltaSeconds)
			if tc.expectErr != nil {
				if !errors.Is(err, tc.expectErr) {
					t.Fatalf("expected %v, got %v", tc.expectErr, err)
				}
				// Also verify SetWakeAlarm rejects it
				setErr := mgr.SetWakeAlarm(tc.deltaSeconds)
				if !errors.Is(setErr, tc.expectErr) {
					t.Fatalf("SetWakeAlarm expected %v, got %v", tc.expectErr, setErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestSetWakeAlarmAt(t *testing.T) {
	mem, _ := setupRTCEnv(t, "", "")
	mgr := rtc.NewManager(mem)

	t.Run("rejects target in past", func(t *testing.T) {
		past := mem.CurrentTime.Add(-10 * time.Minute)
		err := mgr.SetWakeAlarmAt(past)
		if !errors.Is(err, rtc.ErrAlarmInPast) {
			t.Fatalf("expected ErrAlarmInPast, got %v", err)
		}
	})

	t.Run("rejects target inside 180s safety buffer", func(t *testing.T) {
		tooSoon := mem.CurrentTime.Add(120 * time.Second)
		err := mgr.SetWakeAlarmAt(tooSoon)
		if !errors.Is(err, rtc.ErrAlarmTooSoon) {
			t.Fatalf("expected ErrAlarmTooSoon, got %v", err)
		}
	})

	t.Run("accepts target beyond safety buffer", func(t *testing.T) {
		validTarget := mem.CurrentTime.Add(1 * time.Hour)
		if err := mgr.SetWakeAlarmAt(validTarget); err != nil {
			t.Fatalf("SetWakeAlarmAt failed: %v", err)
		}

		status, err := mgr.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected status to be enabled")
		}
		if status.Target.Unix() != validTarget.Unix() {
			t.Fatalf("expected target %v, got %v", validTarget, status.Target)
		}
	})
}

func TestGetStatusUTCAndLocal(t *testing.T) {
	t.Run("disabled alarm", func(t *testing.T) {
		mem, _ := setupRTCEnv(t, "", "")
		mgr := rtc.NewManager(mem)

		status, err := mgr.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		if status.Enabled {
			t.Fatal("expected alarm to be disabled")
		}
		if status.ClockMode != rtc.ClockModeUTC {
			t.Fatalf("expected ClockModeUTC, got %q", status.ClockMode)
		}
	})

	t.Run("active alarm with UTC adjtime", func(t *testing.T) {
		wakeEpoch := int64(1729340000)
		mem, _ := setupRTCEnv(t, fmt.Sprintf("%d\n", wakeEpoch), "0.0 0 0\n0\nUTC\n")
		mgr := rtc.NewManager(mem)

		status, err := mgr.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected alarm to be enabled")
		}
		if status.ClockMode != rtc.ClockModeUTC {
			t.Fatalf("expected ClockModeUTC, got %q", status.ClockMode)
		}
		if status.Target.Unix() != wakeEpoch {
			t.Fatalf("expected target epoch %d, got %d", wakeEpoch, status.Target.Unix())
		}
	})

	t.Run("active alarm with LOCAL adjtime", func(t *testing.T) {
		// When BIOS is in local time (e.g. UTC+7 Jakarta),
		// the RTC hardware registers contain 2026-10-19 17:00:00.
		// The Linux kernel treats these fields as UTC to produce sysfs wakealarm:
		// mktime64(2026, 10, 19, 17, 0, 0)
		rtcCalendarUTC := time.Date(2026, 10, 19, 17, 0, 0, 0, time.UTC)
		wakeEpoch := rtcCalendarUTC.Unix()

		jakarta := time.FixedZone("WIB", 7*3600)

		mem, _ := setupRTCEnv(t, fmt.Sprintf("%d\n", wakeEpoch), "0.0 0 0\n0\nLOCAL\n")
		mgr := rtc.NewManager(mem, rtc.WithLocation(jakarta))

		status, err := mgr.GetStatus()
		if err != nil {
			t.Fatalf("GetStatus failed: %v", err)
		}
		if !status.Enabled {
			t.Fatal("expected alarm to be enabled")
		}
		if status.ClockMode != rtc.ClockModeLocal {
			t.Fatalf("expected ClockModeLocal, got %q", status.ClockMode)
		}

		// The target in local time must correspond to 17:00:00 on 2026-10-19 in Jakarta
		expectedLocal := time.Date(2026, 10, 19, 17, 0, 0, 0, jakarta)
		if !status.Target.Equal(expectedLocal) {
			t.Fatalf("expected target %v, got %v", expectedLocal, status.Target)
		}
	})

	t.Run("missing wakealarm file returns ErrAlarmNotSupported", func(t *testing.T) {
		mem := env.NewMemEnv()
		mgr := rtc.NewManager(mem)

		_, err := mgr.GetStatus()
		if !errors.Is(err, rtc.ErrAlarmNotSupported) {
			t.Fatalf("expected ErrAlarmNotSupported, got %v", err)
		}
	})

	t.Run("malformed epoch returns error", func(t *testing.T) {
		mem, _ := setupRTCEnv(t, "not-a-number\n", "")
		mgr := rtc.NewManager(mem)

		_, err := mgr.GetStatus()
		if err == nil {
			t.Fatal("expected error parsing malformed epoch")
		}
	})
}

func TestExecutePowerAction(t *testing.T) {
	ctx := context.Background()

	t.Run("executes valid power actions via systemctl", func(t *testing.T) {
		validActions := []string{"poweroff", "suspend", "hibernate"}

		for _, action := range validActions {
			mem := env.NewMemEnv()
			var recordedCommand string
			var recordedArgs []string

			mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				recordedCommand = name
				recordedArgs = args
				return []byte("ok"), nil
			}

			mgr := rtc.NewManager(mem)
			if err := mgr.ExecutePowerAction(ctx, action); err != nil {
				t.Fatalf("ExecutePowerAction(%q) failed: %v", action, err)
			}

			if recordedCommand != "systemctl" {
				t.Fatalf("expected systemctl command, got %q", recordedCommand)
			}
			if len(recordedArgs) != 1 || recordedArgs[0] != action {
				t.Fatalf("expected args [%s], got %v", action, recordedArgs)
			}
		}
	})

	t.Run("rejects invalid power actions without executing", func(t *testing.T) {
		invalidActions := []string{"reboot", "halt", "shutdown", "poweroff; rm -rf /", ""}

		for _, action := range invalidActions {
			mem := env.NewMemEnv()
			executed := false
			mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				executed = true
				return nil, nil
			}

			mgr := rtc.NewManager(mem)
			err := mgr.ExecutePowerAction(ctx, action)
			if !errors.Is(err, rtc.ErrInvalidPowerAction) {
				t.Fatalf("action %q expected ErrInvalidPowerAction, got %v", action, err)
			}
			if executed {
				t.Fatalf("action %q triggered execution despite being invalid", action)
			}
		}
	})

	t.Run("returns wrapped error when systemctl fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("failed to connect to bus"), errors.New("exit status 1")
		}

		mgr := rtc.NewManager(mem)
		err := mgr.ExecutePowerAction(ctx, "poweroff")
		if err == nil {
			t.Fatal("expected error when systemctl fails")
		}
		if !strings.Contains(err.Error(), "systemctl") {
			t.Fatalf("expected error mentioning systemctl, got %v", err)
		}
	})
}

func TestCustomOptions(t *testing.T) {
	mem := env.NewMemEnv()
	customRTC := "/custom/sys/class/rtc/rtc1"
	customAdj := "/custom/etc/adjtime"

	mem.Files[filepath.Join(customRTC, "wakealarm")] = []byte("1729340000\n")
	mem.Files[customAdj] = []byte("0.0 0 0\n0\nLOCAL\n")

	tokyo := time.FixedZone("JST", 9*3600)
	mgr := rtc.NewManager(mem,
		rtc.WithSysfsPath(customRTC),
		rtc.WithAdjtimePath(customAdj),
		rtc.WithLocation(tokyo),
	)

	status, err := mgr.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Device != customRTC {
		t.Fatalf("expected device %q, got %q", customRTC, status.Device)
	}
	if status.ClockMode != rtc.ClockModeLocal {
		t.Fatalf("expected ClockModeLocal, got %q", status.ClockMode)
	}
	if status.Target.Location() != tokyo {
		t.Fatalf("expected location Tokyo, got %v", status.Target.Location())
	}
}

func TestManager_EnvironmentVariableRTCDevice(t *testing.T) {
	mem := env.NewMemEnv()
	customRTC := "/sys/class/rtc/rtc2"
	mem.EnvVars["ARKON_RTC_DEVICE"] = customRTC
	mem.Files[filepath.Join(customRTC, "wakealarm")] = []byte("1729340000\n")

	mgr := rtc.NewManager(mem)
	status, err := mgr.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.Device != customRTC {
		t.Fatalf("expected device %q from ARKON_RTC_DEVICE, got %q", customRTC, status.Device)
	}
}

func TestManager_VerboseLogging(t *testing.T) {
	mem, _ := setupRTCEnv(t, "1700000000\n", "")
	var buf strings.Builder
	mgr := rtc.NewManager(mem,
		rtc.WithVerbose(true),
		rtc.WithLogger(&buf),
	)

	if err := mgr.ClearAlarm(); err != nil {
		t.Fatalf("ClearAlarm failed: %v", err)
	}
	if !strings.Contains(buf.String(), "[rtc] clearing alarm on /sys/class/rtc/rtc0/wakealarm") {
		t.Errorf("expected clearing alarm log, got: %s", buf.String())
	}

	buf.Reset()
	if err := mgr.SetWakeAlarm(600); err != nil {
		t.Fatalf("SetWakeAlarm failed: %v", err)
	}
	logOut := buf.String()
	if !strings.Contains(logOut, "[rtc] programming alarm on /sys/class/rtc/rtc0/wakealarm: writing relative +600s") {
		t.Errorf("expected programming alarm log, got: %s", logOut)
	}
	if !strings.Contains(logOut, "[rtc] hardware retention verified: epoch") {
		t.Errorf("expected retention verified log, got: %s", logOut)
	}
}
