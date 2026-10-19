package lifecycle_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/lifecycle"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/rtc"
	"github.com/adrianfinantyo/arkon/internal/state"
	"github.com/adrianfinantyo/arkon/internal/teardown"
)

type mockProvider struct {
	stopTierCalls    []string
	stopResidualCall int
	stopTierFunc     func(ctx context.Context, tier string, timeout time.Duration) error
	stopResidualFunc func(ctx context.Context, timeout time.Duration) error
}

func (m *mockProvider) Name() string {
	return "mock-teardown"
}

func (m *mockProvider) CheckLiveness(ctx context.Context) error {
	return nil
}

func (m *mockProvider) StopTier(ctx context.Context, tier string, timeout time.Duration) error {
	m.stopTierCalls = append(m.stopTierCalls, tier)
	if m.stopTierFunc != nil {
		return m.stopTierFunc(ctx, tier, timeout)
	}
	return nil
}

func (m *mockProvider) StopResidual(ctx context.Context, timeout time.Duration) error {
	m.stopResidualCall++
	if m.stopResidualFunc != nil {
		return m.stopResidualFunc(ctx, timeout)
	}
	return nil
}

func setupTestEnv(t *testing.T, baseTime time.Time) (*env.MemEnv, []string) {
	t.Helper()
	mem := env.NewMemEnv()
	mem.CurrentTime = baseTime

	var execLog []string
	wakePath := "/sys/class/rtc/rtc0/wakealarm"
	memPath := "/proc/meminfo"

	mem.Files[wakePath] = []byte("0\n")
	mem.Files[memPath] = []byte("MemTotal: 4000000 kB\nDirty: 0 kB\n")
	mem.Files["/etc/adjtime"] = []byte("0.0 0 0\n0\nUTC\n")

	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
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
					return err
				}
				targetEpoch := mem.CurrentTime.Unix() + sec
				mem.Files[path] = []byte(fmt.Sprintf("%d\n", targetEpoch))
				return nil
			}
		}
		mem.Files[path] = data
		return nil
	}

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdLine := name + " " + strings.Join(args, " ")
		execLog = append(execLog, cmdLine)
		return []byte("ok"), nil
	}

	return mem, execLog
}

func validTestManifest() *manifest.Manifest {
	m := &manifest.Manifest{
		Version: "1",
		Host: manifest.HostConfig{
			Timezone:    "UTC",
			RTCDevice:   "/sys/class/rtc/rtc0",
			PowerAction: "poweroff",
		},
		Circadian: manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		},
		Teardown: manifest.TeardownConfig{
			Provider:       "docker-compose",
			TimeoutSeconds: 10,
			ProfileOrder:   []string{"apps", "infra"},
		},
	}
	m.ApplyDefaults()
	return m
}

func TestCalculateNextWake(t *testing.T) {
	// Base: Monday 2026-10-19 23:00:00 UTC
	base := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)

	t.Run("computes tomorrow morning wake time", func(t *testing.T) {
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    manifest.DefaultDays,
		}
		nextWake, err := lifecycle.CalculateNextWake(base, cfg, "UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC)
		if !nextWake.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, nextWake)
		}
	})

	t.Run("computes same day wake if before wake_at and exceeds safety buffer", func(t *testing.T) {
		morning := time.Date(2026, 10, 19, 5, 0, 0, 0, time.UTC)
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    manifest.DefaultDays,
		}
		nextWake, err := lifecycle.CalculateNextWake(morning, cfg, "UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := time.Date(2026, 10, 19, 7, 0, 0, 0, time.UTC)
		if !nextWake.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, nextWake)
		}
	})

	t.Run("skips today if within 180s safety buffer", func(t *testing.T) {
		tooClose := time.Date(2026, 10, 19, 6, 58, 0, 0, time.UTC)
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    manifest.DefaultDays,
		}
		nextWake, err := lifecycle.CalculateNextWake(tooClose, cfg, "UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Since 06:58 is only 120s before 07:00, today at 07:00 is skipped; next day at 07:00 selected
		expected := time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC)
		if !nextWake.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, nextWake)
		}
	})

	t.Run("skips inactive days", func(t *testing.T) {
		// Friday night 23:00
		fridayNight := time.Date(2026, 10, 23, 23, 0, 0, 0, time.UTC)
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
			Days:    []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		}
		nextWake, err := lifecycle.CalculateNextWake(fridayNight, cfg, "UTC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Saturday and Sunday skipped; next wake is Monday 2026-10-26 07:00
		expected := time.Date(2026, 10, 26, 7, 0, 0, 0, time.UTC)
		if !nextWake.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, nextWake)
		}
	})

	t.Run("rejects invalid timezone", func(t *testing.T) {
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "07:00",
		}
		_, err := lifecycle.CalculateNextWake(base, cfg, "Invalid/TimeZone")
		if err == nil {
			t.Fatal("expected error for invalid timezone")
		}
	})

	t.Run("rejects malformed wake_at", func(t *testing.T) {
		cfg := manifest.CircadianConfig{
			SleepAt: "23:00",
			WakeAt:  "25:99",
		}
		_, err := lifecycle.CalculateNextWake(base, cfg, "UTC")
		if err == nil {
			t.Fatal("expected error for malformed wake_at")
		}
	})
}

func TestCoordinator_ExecuteFullPipeline(t *testing.T) {
	// 23:00 UTC on Monday
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)

	m := validTestManifest()
	m.Hooks.PreSleep = []manifest.HookConfig{
		{
			Name:           "pre-backup",
			Command:        "backup-db",
			TimeoutSeconds: 5,
		},
	}

	mockP := &mockProvider{}
	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))

	var events []lifecycle.Event
	listener := func(e lifecycle.Event) {
		events = append(events, e)
	}

	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
		lifecycle.WithEventListener(listener),
	)

	result, err := coordinator.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected coordinator execution failure: %v", err)
	}

	if result.DryRun {
		t.Fatal("expected real execution, got dry run")
	}
	if !result.DiskSynced {
		t.Fatal("expected disk synced")
	}
	if !result.WakeAlarmArmed {
		t.Fatal("expected wake alarm armed")
	}
	if result.PowerAction != "poweroff" {
		t.Fatalf("expected power action poweroff, got %s", result.PowerAction)
	}

	// Verify workload teardown tiers stopped in order
	if len(mockP.stopTierCalls) != 2 || mockP.stopTierCalls[0] != "apps" || mockP.stopTierCalls[1] != "infra" {
		t.Fatalf("unexpected stop tier calls: %v", mockP.stopTierCalls)
	}
	if mockP.stopResidualCall != 1 {
		t.Fatalf("expected 1 stop residual call, got %d", mockP.stopResidualCall)
	}

	// Verify wakealarm sysfs content has target epoch
	alarmData, err := mem.ReadFile("/sys/class/rtc/rtc0/wakealarm")
	if err != nil {
		t.Fatalf("failed to read sysfs wakealarm: %v", err)
	}
	alarmStr := strings.TrimSpace(string(alarmData))
	if alarmStr == "" || alarmStr == "0" {
		t.Fatalf("sysfs wakealarm was not programmed: %q", alarmStr)
	}

	// Verify events were emitted
	eventTypes := make([]lifecycle.EventType, len(events))
	for i, e := range events {
		eventTypes[i] = e.Type
	}
	expectedEvents := []lifecycle.EventType{
		lifecycle.EventInvariantsChecked,
		lifecycle.EventPreSleepHooksStarted,
		lifecycle.EventPreSleepHookStarted,
		lifecycle.EventPreSleepHookCompleted,
		lifecycle.EventPreSleepHooksCompleted,
		lifecycle.EventTeardownStarted,
		lifecycle.EventTeardownCompleted,
		lifecycle.EventRTCArmed,
		lifecycle.EventPowerActionStarted,
		lifecycle.EventPowerActionCompleted,
	}
	for _, expectedType := range expectedEvents {
		found := false
		for _, actualType := range eventTypes {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected event %s in event list: %v", expectedType, eventTypes)
		}
	}
}

func TestCoordinator_DryRun(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem := env.NewMemEnv()
	mem.CurrentTime = baseTime

	// No mock files populated in mem
	m := validTestManifest()
	m.Hooks.PreSleep = []manifest.HookConfig{
		{
			Name:           "should-not-run",
			Command:        "exit 1",
			TimeoutSeconds: 5,
		},
	}

	mockP := &mockProvider{
		stopTierFunc: func(ctx context.Context, tier string, timeout time.Duration) error {
			t.Fatal("StopTier should not be called during dry run")
			return nil
		},
	}
	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
		lifecycle.WithDryRun(true),
	)

	result, err := coordinator.Execute(context.Background())
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !result.DryRun {
		t.Fatal("expected result.DryRun to be true")
	}
	if len(result.TiersStopped) != 2 {
		t.Fatalf("expected simulated 2 tiers stopped, got %v", result.TiersStopped)
	}
	if len(mockP.stopTierCalls) != 0 {
		t.Fatalf("expected 0 tier stops in dry run, got %d", len(mockP.stopTierCalls))
	}
}

func TestCoordinator_DaytimeProtectionInhibition(t *testing.T) {
	// 14:00 UTC on Monday (inside daytime window 07:00 - 23:00)
	daytime := time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, daytime)
	m := validTestManifest()

	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return daytime }))

	t.Run("aborts without force", func(t *testing.T) {
		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return daytime }),
			lifecycle.WithStateManager(stateMgr),
		)

		_, err := coordinator.Execute(context.Background())
		if err == nil {
			t.Fatal("expected daytime inhibition error")
		}

		var inErr *lifecycle.InhibitionError
		if !errors.As(err, &inErr) {
			t.Fatalf("expected *lifecycle.InhibitionError, got %T: %v", err, err)
		}
		if inErr.Result.Code != "ERR_DAYTIME_PROTECTION" {
			t.Fatalf("expected ERR_DAYTIME_PROTECTION, got %s", inErr.Result.Code)
		}
	})

	t.Run("proceeds with force", func(t *testing.T) {
		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return daytime }),
			lifecycle.WithStateManager(stateMgr),
			lifecycle.WithForce(true),
			lifecycle.WithDryRun(true),
		)

		result, err := coordinator.Execute(context.Background())
		if err != nil {
			t.Fatalf("expected success with force, got %v", err)
		}
		if !result.Forced {
			t.Fatal("expected result.Forced to be true")
		}
	})
}

func TestCoordinator_ActiveLockInhibition(t *testing.T) {
	// 23:00 UTC on Monday (outside daytime window)
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()

	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))
	_, err := stateMgr.AcquireLock("Database reindex", 2*time.Hour, "dba")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	t.Run("aborts without force", func(t *testing.T) {
		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return baseTime }),
			lifecycle.WithStateManager(stateMgr),
		)

		_, err := coordinator.Execute(context.Background())
		if err == nil {
			t.Fatal("expected active lock inhibition error")
		}

		var inErr *lifecycle.InhibitionError
		if !errors.As(err, &inErr) {
			t.Fatalf("expected *lifecycle.InhibitionError, got %T: %v", err, err)
		}
		if inErr.Result.Code != "ERR_INHIBITED" {
			t.Fatalf("expected ERR_INHIBITED, got %s", inErr.Result.Code)
		}
		if inErr.Result.Lock == nil || inErr.Result.Lock.Reason != "Database reindex" {
			t.Fatalf("unexpected lock details: %v", inErr.Result.Lock)
		}
	})

	t.Run("proceeds with force", func(t *testing.T) {
		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return baseTime }),
			lifecycle.WithStateManager(stateMgr),
			lifecycle.WithForce(true),
			lifecycle.WithDryRun(true),
		)

		result, err := coordinator.Execute(context.Background())
		if err != nil {
			t.Fatalf("expected success with force, got %v", err)
		}
		if !result.Forced {
			t.Fatal("expected result.Forced to be true")
		}
	})
}

func TestCoordinator_PreSleepHookFailure(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)

	t.Run("aborts on hook failure when ignore_errors is false", func(t *testing.T) {
		m := validTestManifest()
		m.Hooks.PreSleep = []manifest.HookConfig{
			{
				Name:           "failing-hook",
				Command:        "fail-cmd",
				TimeoutSeconds: 5,
				IgnoreErrors:   false,
			},
		}

		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("error"), errors.New("command failed with code 1")
		}

		mockP := &mockProvider{}
		coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
		stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return baseTime }),
			lifecycle.WithTeardownCoordinator(coordTeardown),
			lifecycle.WithStateManager(stateMgr),
		)

		_, err := coordinator.Execute(context.Background())
		if err == nil {
			t.Fatal("expected error on pre-sleep hook failure")
		}
		if !strings.Contains(err.Error(), "failing-hook") {
			t.Fatalf("expected error mentioning hook name, got %v", err)
		}
		// Confirm workload teardown completed before pre-sleep hook ran
		if len(mockP.stopTierCalls) == 0 {
			t.Fatalf("expected workload teardown to run before pre-sleep hooks")
		}
		// Confirm wakealarm was not programmed
		alarmData, _ := mem.ReadFile("/sys/class/rtc/rtc0/wakealarm")
		alarmStr := strings.TrimSpace(string(alarmData))
		if alarmStr != "" && alarmStr != "0" {
			t.Fatalf("wakealarm should not have been armed, got %q", alarmStr)
		}
	})

	t.Run("continues on hook failure when ignore_errors is true", func(t *testing.T) {
		m := validTestManifest()
		m.Hooks.PreSleep = []manifest.HookConfig{
			{
				Name:           "optional-hook",
				Command:        "fail-cmd",
				TimeoutSeconds: 5,
				IgnoreErrors:   true,
			},
		}

		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if strings.Contains(strings.Join(args, " "), "fail-cmd") {
				return []byte("error"), errors.New("command failed with code 1")
			}
			return []byte("ok"), nil
		}

		mockP := &mockProvider{}
		coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
		stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

		coordinator := lifecycle.NewCoordinator(mem, m,
			lifecycle.WithClock(func() time.Time { return baseTime }),
			lifecycle.WithTeardownCoordinator(coordTeardown),
			lifecycle.WithStateManager(stateMgr),
		)

		result, err := coordinator.Execute(context.Background())
		if err != nil {
			t.Fatalf("expected success with ignore_errors: true, got %v", err)
		}
		if !result.WakeAlarmArmed {
			t.Fatal("expected wakealarm armed")
		}
	})
}

func TestCoordinator_WorkloadTeardownFailureAborts(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()

	mockP := &mockProvider{
		stopTierFunc: func(ctx context.Context, tier string, timeout time.Duration) error {
			if tier == "apps" {
				return errors.New("container jellyfin failed to stop: timeout")
			}
			return nil
		},
	}

	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
	)

	_, err := coordinator.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error on teardown failure")
	}
	if !strings.Contains(err.Error(), "jellyfin") {
		t.Fatalf("expected error mentioning container failure, got %v", err)
	}

	// Confirm wakealarm was never programmed because teardown aborted
	alarmData, _ := mem.ReadFile("/sys/class/rtc/rtc0/wakealarm")
	if strings.TrimSpace(string(alarmData)) != "0" && strings.TrimSpace(string(alarmData)) != "" {
		t.Fatalf("wakealarm should not have been programmed on failure, got %q", string(alarmData))
	}
}

func TestCoordinator_RTCVerificationFailureAborts(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()

	// Make RTC wakealarm readback fail retention check
	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
		if path == "/sys/class/rtc/rtc0/wakealarm" {
			mem.Files[path] = []byte("") // Empty readback fails retention check
			return nil
		}
		mem.Files[path] = data
		return nil
	}

	mockP := &mockProvider{}
	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
	)

	_, err := coordinator.Execute(context.Background())
	if err == nil {
		t.Fatal("expected error on RTC retention failure")
	}
	if !errors.Is(err, rtc.ErrAlarmRetentionFailed) && !strings.Contains(err.Error(), "retention") {
		t.Fatalf("expected retention failure error, got %v", err)
	}
}

func TestCoordinator_ExecutionOrder(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()

	m.Hooks.PreSleep = []manifest.HookConfig{
		{Name: "pre-sleep-hook", Command: "echo pre-sleep", TimeoutSeconds: 5},
	}

	var trace []string
	mockP := &mockProvider{
		stopTierFunc: func(ctx context.Context, tier string, timeout time.Duration) error {
			trace = append(trace, "stop-tier:"+tier)
			return nil
		},
		stopResidualFunc: func(ctx context.Context, timeout time.Duration) error {
			trace = append(trace, "stop-residual")
			return nil
		},
	}

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		cmdLine := name + " " + strings.Join(args, " ")
		trace = append(trace, "exec:"+cmdLine)
		return []byte("ok"), nil
	}

	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	var eventTypes []lifecycle.EventType
	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
		lifecycle.WithEventListener(func(e lifecycle.Event) {
			eventTypes = append(eventTypes, e.Type)
		}),
	)

	result, err := coordinator.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected Execute error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected status completed, got %s", result.Status)
	}

	// Verify sequential event order: Teardown before PreSleepHooks
	var teardownIdx, preSleepIdx, rtcIdx, powerIdx int
	for i, evt := range eventTypes {
		switch evt {
		case lifecycle.EventTeardownCompleted:
			teardownIdx = i
		case lifecycle.EventPreSleepHooksStarted:
			preSleepIdx = i
		case lifecycle.EventRTCArmed:
			rtcIdx = i
		case lifecycle.EventPowerActionStarted:
			powerIdx = i
		}
	}

	if !(teardownIdx < preSleepIdx && preSleepIdx < rtcIdx && rtcIdx < powerIdx) {
		t.Fatalf("expected order Teardown < PreSleep < RTC < Power, got indices: teardown=%d, preSleep=%d, rtc=%d, power=%d",
			teardownIdx, preSleepIdx, rtcIdx, powerIdx)
	}
}

func TestCoordinator_ExecutePostWake(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 7, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()
	m.Hooks.PostWake = []manifest.HookConfig{
		{Name: "wake-hook-1", Command: "echo wake 1", TimeoutSeconds: 5},
		{Name: "wake-hook-2", Command: "echo wake 2", TimeoutSeconds: 5},
	}

	var executed []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executed = append(executed, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	var events []lifecycle.EventType
	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithEventListener(func(e lifecycle.Event) {
			events = append(events, e.Type)
		}),
	)

	err := coordinator.ExecutePostWake(context.Background())
	if err != nil {
		t.Fatalf("ExecutePostWake failed: %v", err)
	}
	if len(executed) != 2 {
		t.Fatalf("expected 2 hooks executed, got %d: %v", len(executed), executed)
	}

	expectedEvents := []lifecycle.EventType{
		lifecycle.EventPostWakeHooksStarted,
		lifecycle.EventPostWakeHookStarted,
		lifecycle.EventPostWakeHookCompleted,
		lifecycle.EventPostWakeHookStarted,
		lifecycle.EventPostWakeHookCompleted,
		lifecycle.EventPostWakeHooksCompleted,
	}
	if len(events) != len(expectedEvents) {
		t.Fatalf("expected %d events, got %d: %v", len(expectedEvents), len(events), events)
	}
}

func TestCoordinator_PostWakeOnSuspend(t *testing.T) {
	baseTime := time.Date(2026, 10, 19, 23, 0, 0, 0, time.UTC)
	mem, _ := setupTestEnv(t, baseTime)
	m := validTestManifest()
	m.Host.PowerAction = "suspend"
	m.Hooks.PostWake = []manifest.HookConfig{
		{Name: "post-suspend-wake", Command: "echo resumed", TimeoutSeconds: 5},
	}

	var executed []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executed = append(executed, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	mockP := &mockProvider{}
	coordTeardown := teardown.NewCoordinator(mem, m, teardown.WithProvider(mockP))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return baseTime }))

	coordinator := lifecycle.NewCoordinator(mem, m,
		lifecycle.WithClock(func() time.Time { return baseTime }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
	)

	result, err := coordinator.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected Execute error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected status completed, got %s", result.Status)
	}

	foundPostWakeHook := false
	for _, cmd := range executed {
		if strings.Contains(cmd, "resumed") {
			foundPostWakeHook = true
			break
		}
	}
	if !foundPostWakeHook {
		t.Fatalf("expected post-wake hook to execute after suspend resume, got commands: %v", executed)
	}
}

func TestCoordinator_DaytimeWindowOverride(t *testing.T) {
	// Circadian is 07:00 wake, 23:00 sleep.
	// Explicit daytime window is 09:00 to 17:00.
	m := validTestManifest()
	m.Inhibition.DaytimeStart = "09:00"
	m.Inhibition.DaytimeEnd = "17:00"

	// 1. Test at 08:00 (outside 09:00-17:00, allowed)
	timeAt8AM := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	memAt8, _ := setupTestEnv(t, timeAt8AM)
	coordTeardown := teardown.NewCoordinator(memAt8, m, teardown.WithProvider(&mockProvider{}))
	stateMgr := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return timeAt8AM }))

	c8 := lifecycle.NewCoordinator(memAt8, m,
		lifecycle.WithClock(func() time.Time { return timeAt8AM }),
		lifecycle.WithTeardownCoordinator(coordTeardown),
		lifecycle.WithStateManager(stateMgr),
		lifecycle.WithDryRun(true),
	)

	res, err := c8.Execute(context.Background())
	if err != nil {
		t.Fatalf("expected sleep to be allowed at 08:00 with daytime_start 09:00, got: %v", err)
	}
	if !res.DryRun {
		t.Fatal("expected DryRun to be true")
	}

	// 2. Test at 10:00 (inside 09:00-17:00, blocked)
	timeAt10AM := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	memAt10, _ := setupTestEnv(t, timeAt10AM)
	coordTeardown10 := teardown.NewCoordinator(memAt10, m, teardown.WithProvider(&mockProvider{}))
	stateMgr10 := state.NewManager(t.TempDir(), state.WithClock(func() time.Time { return timeAt10AM }))

	c10 := lifecycle.NewCoordinator(memAt10, m,
		lifecycle.WithClock(func() time.Time { return timeAt10AM }),
		lifecycle.WithTeardownCoordinator(coordTeardown10),
		lifecycle.WithStateManager(stateMgr10),
		lifecycle.WithDryRun(true),
	)

	_, err = c10.Execute(context.Background())
	if err == nil {
		t.Fatal("expected sleep to be blocked at 10:00 within daytime window 09:00-17:00")
	}
	var inErr *lifecycle.InhibitionError
	if !errors.As(err, &inErr) || inErr.Result.Code != "ERR_DAYTIME_PROTECTION" {
		t.Fatalf("expected ERR_DAYTIME_PROTECTION, got: %v", err)
	}
}

