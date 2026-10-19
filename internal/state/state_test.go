package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestAcquireLock(t *testing.T) {
	dir := t.TempDir()
	fixedNow := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	mgr := NewManager(dir, WithClock(func() time.Time { return fixedNow }))

	lock, err := mgr.AcquireLock("Database maintenance", 2*time.Hour, "adrian")
	if err != nil {
		t.Fatalf("unexpected error acquiring lock: %v", err)
	}

	if lock.Reason != "Database maintenance" {
		t.Errorf("expected reason 'Database maintenance', got %q", lock.Reason)
	}
	if lock.CreatedBy != "adrian" {
		t.Errorf("expected created_by 'adrian', got %q", lock.CreatedBy)
	}
	expectedExpiry := fixedNow.Add(2 * time.Hour)
	if !lock.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, lock.ExpiresAt)
	}
	if lock.ID == "" {
		t.Error("expected non-empty lock ID")
	}

	// Verify file was written to state.json
	statePath := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state.json: %v", err)
	}

	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to unmarshal state.json: %v", err)
	}

	if len(saved.Locks) != 1 {
		t.Fatalf("expected 1 lock in state, got %d", len(saved.Locks))
	}
	if saved.Locks[0].ID != lock.ID {
		t.Errorf("expected saved lock ID %q, got %q", lock.ID, saved.Locks[0].ID)
	}
}

func TestListLocksAndFormatRemaining(t *testing.T) {
	dir := t.TempDir()
	fixedNow := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	mgr := NewManager(dir, WithClock(func() time.Time { return fixedNow }))

	// Initially empty
	locks, err := mgr.ListLocks()
	if err != nil {
		t.Fatalf("unexpected error listing locks: %v", err)
	}
	if len(locks) != 0 {
		t.Fatalf("expected 0 locks, got %d", len(locks))
	}

	// Acquire lock with 90 minute duration
	_, err = mgr.AcquireLock("Kernel update", 90*time.Minute, "root")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	locks, err = mgr.ListLocks()
	if err != nil {
		t.Fatalf("failed to list locks: %v", err)
	}
	if len(locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(locks))
	}
	if locks[0].Remaining != "1h 30m" {
		t.Errorf("expected remaining '1h 30m', got %q", locks[0].Remaining)
	}
}

func TestReleaseLock(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	lock, err := mgr.AcquireLock("Backup", 1*time.Hour, "operator")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Release existing lock
	if err := mgr.ReleaseLock(lock.ID); err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Verify state is empty
	locks, err := mgr.ListLocks()
	if err != nil {
		t.Fatalf("failed to list locks: %v", err)
	}
	if len(locks) != 0 {
		t.Fatalf("expected 0 locks after release, got %d", len(locks))
	}

	// Releasing non-existent lock should error
	err = mgr.ReleaseLock("non-existent-id")
	if err == nil {
		t.Fatal("expected error releasing non-existent lock, got nil")
	}
}

func TestAutoPruneExpiredLocksOnRead(t *testing.T) {
	dir := t.TempDir()
	currentTime := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	mgr := NewManager(dir, WithClock(func() time.Time { return currentTime }))

	// Acquire a lock valid for 30 minutes
	lock1, err := mgr.AcquireLock("Short task", 30*time.Minute, "adrian")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Acquire a lock valid for 2 hours
	lock2, err := mgr.AcquireLock("Long task", 2*time.Hour, "adrian")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Advance clock by 45 minutes (lock1 expired, lock2 active)
	currentTime = currentTime.Add(45 * time.Minute)

	// ListLocks should automatically prune lock1
	activeLocks, err := mgr.ListLocks()
	if err != nil {
		t.Fatalf("failed to list locks: %v", err)
	}

	if len(activeLocks) != 1 {
		t.Fatalf("expected 1 active lock, got %d", len(activeLocks))
	}
	if activeLocks[0].ID != lock2.ID {
		t.Errorf("expected active lock %s, got %s", lock2.ID, activeLocks[0].ID)
	}

	// Verify state.json on disk was updated to remove lock1
	statePath := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}
	var diskState State
	if err := json.Unmarshal(data, &diskState); err != nil {
		t.Fatalf("failed to unmarshal state: %v", err)
	}
	if len(diskState.Locks) != 1 {
		t.Fatalf("expected disk state to have 1 lock, got %d", len(diskState.Locks))
	}
	if diskState.Locks[0].ID != lock2.ID {
		t.Errorf("expected disk lock %s, got %s", lock2.ID, diskState.Locks[0].ID)
	}
	_ = lock1
}

func TestConcurrentFlock(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	const workerCount = 10
	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			defer wg.Done()
			_, err := mgr.AcquireLock("Concurrent worker", 1*time.Hour, "worker")
			if err != nil {
				t.Errorf("worker %d failed to acquire lock: %v", workerID, err)
			}
		}(i)
	}

	wg.Wait()

	locks, err := mgr.ListLocks()
	if err != nil {
		t.Fatalf("failed to list locks: %v", err)
	}
	if len(locks) != workerCount {
		t.Fatalf("expected %d locks after concurrent acquisitions, got %d", workerCount, len(locks))
	}
}

func TestAtomicWriteTempFileRenamed(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	_, err := mgr.AcquireLock("Test atomic", 1*time.Hour, "tester")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Ensure no leftover temporary files in directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name != "state.json" && name != "state.lock" {
			t.Errorf("unexpected temporary file found: %s", name)
		}
	}
}

func TestFileLockExclusivity(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "state.lock")

	f1, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("failed to open lock file: %v", err)
	}
	defer f1.Close()

	if err := syscall.Flock(int(f1.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("failed to acquire exclusive flock: %v", err)
	}

	// Try acquiring non-blocking flock from second handle; should fail with EWOULDBLOCK
	f2, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("failed to open second lock file: %v", err)
	}
	defer f2.Close()

	err = syscall.Flock(int(f2.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		t.Fatal("expected non-blocking lock to fail on already locked file, got nil")
	}

	// Unlock f1
	if err := syscall.Flock(int(f1.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("failed to unlock: %v", err)
	}

	// Now f2 should succeed
	err = syscall.Flock(int(f2.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		t.Fatalf("expected f2 lock to succeed after f1 unlock: %v", err)
	}
	_ = syscall.Flock(int(f2.Fd()), syscall.LOCK_UN)
}

func TestDaytimeOperationalCalculation(t *testing.T) {
	// Standard daytime schedule: wake 07:00, sleep 23:00
	wakeAt := "07:00"
	sleepAt := "23:00"

	testCases := []struct {
		name       string
		timeStr    string // HH:MM
		isDaytime  bool
	}{
		{"Morning before wake", "06:30", false},
		{"Exactly at wake time", "07:00", true},
		{"Midday", "12:00", true},
		{"Late afternoon", "17:45", true},
		{"One minute before sleep", "22:59", true},
		{"Exactly at sleep time", "23:00", false},
		{"Night after sleep", "23:30", false},
		{"Early morning midnight", "00:00", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tVal, err := time.Parse("15:04", tc.timeStr)
			if err != nil {
				t.Fatalf("failed to parse time: %v", err)
			}
			isDay, err := IsDaytimeOperational(tVal, wakeAt, sleepAt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if isDay != tc.isDaytime {
				t.Errorf("time %s: expected isDaytime=%v, got %v", tc.timeStr, tc.isDaytime, isDay)
			}
		})
	}
}

func TestDaytimeOperationalOvernightSchedule(t *testing.T) {
	// Overnight schedule: wake 20:00, sleep 04:00
	wakeAt := "20:00"
	sleepAt := "04:00"

	testCases := []struct {
		name      string
		timeStr   string
		isDaytime bool
	}{
		{"Awake at night", "22:00", true},
		{"Awake early morning", "02:00", true},
		{"Asleep morning", "05:00", false},
		{"Asleep afternoon", "15:00", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tVal, _ := time.Parse("15:04", tc.timeStr)
			isDay, err := IsDaytimeOperational(tVal, wakeAt, sleepAt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if isDay != tc.isDaytime {
				t.Errorf("time %s: expected isDaytime=%v, got %v", tc.timeStr, tc.isDaytime, isDay)
			}
		})
	}
}

func TestEvaluateSleepAllowed(t *testing.T) {
	daytimeCfg := DaytimeConfig{
		WakeAt:            "07:00",
		SleepAt:           "23:00",
		DaytimeProtection: true,
	}

	daytimeClock := time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)
	nightClock := time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)

	// 1. Daytime with no locks -> blocked by daytime protection
	res := EvaluateSleepAllowed(daytimeClock, daytimeCfg, nil, false)
	if !res.Inhibited {
		t.Error("expected sleep to be inhibited during daytime")
	}
	if res.Code != "ERR_DAYTIME_PROTECTION" {
		t.Errorf("expected ERR_DAYTIME_PROTECTION, got %q", res.Code)
	}

	// 2. Daytime with force -> allowed
	res = EvaluateSleepAllowed(daytimeClock, daytimeCfg, nil, true)
	if res.Inhibited {
		t.Error("expected sleep to be allowed with --force")
	}

	// 3. Night with no locks -> allowed
	res = EvaluateSleepAllowed(nightClock, daytimeCfg, nil, false)
	if res.Inhibited {
		t.Error("expected sleep to be allowed during sleep window with no locks")
	}

	// 4. Night with active lock -> blocked by active lock
	activeLocks := []InhibitionLock{
		{
			ID:        "inh-123",
			Reason:    "Backup in progress",
			CreatedBy: "root",
			ExpiresAt: nightClock.Add(1 * time.Hour),
		},
	}
	res = EvaluateSleepAllowed(nightClock, daytimeCfg, activeLocks, false)
	if !res.Inhibited {
		t.Error("expected sleep to be inhibited by active lock")
	}
	if res.Code != "ERR_INHIBITED" {
		t.Errorf("expected ERR_INHIBITED, got %q", res.Code)
	}
	if res.Lock == nil || res.Lock.ID != "inh-123" {
		t.Errorf("expected lock details populated in result")
	}

	// 5. Night with active lock and force -> allowed
	res = EvaluateSleepAllowed(nightClock, daytimeCfg, activeLocks, true)
	if res.Inhibited {
		t.Error("expected sleep to be allowed with --force even with active lock")
	}
}

func TestFormatRemaining(t *testing.T) {
	cases := []struct {
		d        time.Duration
		expected string
	}{
		{0, "0s"},
		{-5 * time.Second, "0s"},
		{45 * time.Second, "45s"},
		{5 * time.Minute, "5m"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
	}

	for _, tc := range cases {
		out := FormatRemaining(tc.d)
		if out != tc.expected {
			t.Errorf("duration %v: expected %q, got %q", tc.d, tc.expected, out)
		}
	}
}

func TestStateCorruptionResilience(t *testing.T) {
	t.Run("zero-byte state file returns empty state", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state.json")
		if err := os.WriteFile(statePath, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create zero-byte state file: %v", err)
		}

		mgr := NewManager(dir)
		locks, err := mgr.ListLocks()
		if err != nil {
			t.Fatalf("expected nil error on zero-byte state file, got: %v", err)
		}
		if len(locks) != 0 {
			t.Fatalf("expected 0 locks, got %d", len(locks))
		}
	})

	t.Run("whitespace-only state file returns empty state", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state.json")
		if err := os.WriteFile(statePath, []byte("   \n\t  \n"), 0644); err != nil {
			t.Fatalf("failed to create whitespace state file: %v", err)
		}

		mgr := NewManager(dir)
		locks, err := mgr.ListLocks()
		if err != nil {
			t.Fatalf("expected nil error on whitespace state file, got: %v", err)
		}
		if len(locks) != 0 {
			t.Fatalf("expected 0 locks, got %d", len(locks))
		}
	})

	t.Run("malformed state file is quarantined and clean state reset", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state.json")
		corruptContent := []byte(`{"locks": [{"id": "inh-bad", "reason": "broken json...`)
		if err := os.WriteFile(statePath, corruptContent, 0644); err != nil {
			t.Fatalf("failed to write corrupted state file: %v", err)
		}

		warnBuf := &bytes.Buffer{}
		fixedNow := time.Date(2026, 10, 19, 14, 30, 0, 0, time.UTC)
		mgr := NewManager(dir,
			WithClock(func() time.Time { return fixedNow }),
			WithWarningWriter(warnBuf),
		)

		locks, err := mgr.ListLocks()
		if err != nil {
			t.Fatalf("expected nil error after corrupt state recovery, got: %v", err)
		}
		if len(locks) != 0 {
			t.Fatalf("expected 0 locks, got %d", len(locks))
		}

		// Verify warning was logged
		if !strings.Contains(warnBuf.String(), "corrupted state file detected and quarantined") {
			t.Fatalf("expected quarantine warning in output, got: %s", warnBuf.String())
		}

		// Verify quarantine file exists
		expectedQuarantine := filepath.Join(dir, "state.json.corrupt.20261019-143000")
		quarantinedData, err := os.ReadFile(expectedQuarantine)
		if err != nil {
			t.Fatalf("expected quarantine file %s to exist: %v", expectedQuarantine, err)
		}
		if string(quarantinedData) != string(corruptContent) {
			t.Fatalf("quarantined content mismatch: expected %q, got %q", corruptContent, quarantinedData)
		}

		// Verify subsequent acquire lock operation succeeds
		newLock, err := mgr.AcquireLock("Post recovery maintenance", 1*time.Hour, "operator")
		if err != nil {
			t.Fatalf("expected lock acquisition to succeed after corruption recovery: %v", err)
		}
		if newLock.ID == "" {
			t.Fatal("expected valid lock ID")
		}

		activeLocks, err := mgr.ListLocks()
		if err != nil {
			t.Fatalf("expected listing locks to succeed: %v", err)
		}
		if len(activeLocks) != 1 || activeLocks[0].ID != newLock.ID {
			t.Fatalf("expected 1 active lock %s, got %v", newLock.ID, activeLocks)
		}
	})
}

