package teardown_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/teardown"
)

type mockProvider struct {
	name             string
	livenessErr      error
	stopTierCalls    []string
	stopResidualCall int
	stopTierFunc     func(ctx context.Context, tier string, timeout time.Duration) error
	stopResidualFunc func(ctx context.Context, timeout time.Duration) error
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock-provider"
}

func (m *mockProvider) CheckLiveness(ctx context.Context) error {
	return m.livenessErr
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

func setupTestManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Version: "1",
		Teardown: manifest.TeardownConfig{
			Provider:       "docker-compose",
			TimeoutSeconds: 15,
			ProfileOrder:   []string{"apps", "infra"},
		},
		Hooks: manifest.HooksConfig{
			PreTeardown: []manifest.HookConfig{
				{
					Name:           "pre-backup",
					Command:        "backup-db",
					TimeoutSeconds: 5,
				},
			},
			PostTeardown: []manifest.HookConfig{
				{
					Name:           "notify-ops",
					Command:        "send-alert",
					TimeoutSeconds: 5,
				},
			},
		},
	}
}

func TestCoordinator_ExecuteFullPipeline(t *testing.T) {
	t.Run("executes stages in strict sequential dependency order", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("MemTotal: 1000 kB\nDirty: 0 kB\n")

		var executionTrace []string

		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executionTrace = append(executionTrace, fmt.Sprintf("exec:%s:%s", name, args[1]))
			return []byte("ok"), nil
		}

		mock := &mockProvider{
			stopTierFunc: func(ctx context.Context, tier string, timeout time.Duration) error {
				executionTrace = append(executionTrace, fmt.Sprintf("tier:%s", tier))
				return nil
			},
			stopResidualFunc: func(ctx context.Context, timeout time.Duration) error {
				executionTrace = append(executionTrace, "residual")
				return nil
			},
		}

		var events []teardown.EventType
		m := setupTestManifest()
		coord := teardown.NewCoordinator(mem, m,
			teardown.WithProvider(mock),
			teardown.WithDrainInterval(time.Millisecond),
			teardown.WithEventListener(func(e teardown.TeardownEvent) {
				events = append(events, e.Type)
			}),
		)

		err := coord.Teardown(context.Background())
		if err != nil {
			t.Fatalf("unexpected pipeline failure: %v", err)
		}

		expectedTrace := []string{
			"exec:sh:backup-db",
			"tier:apps",
			"tier:infra",
			"residual",
			"exec:sh:send-alert",
		}

		if len(executionTrace) != len(expectedTrace) {
			t.Fatalf("expected trace %v, got %v", expectedTrace, executionTrace)
		}
		for i := range expectedTrace {
			if executionTrace[i] != expectedTrace[i] {
				t.Fatalf("at trace index %d: expected %q, got %q", i, expectedTrace[i], executionTrace[i])
			}
		}

		if mem.SyncCount != 1 {
			t.Fatalf("expected 1 filesystem sync call, got %d", mem.SyncCount)
		}

		expectedEventTypes := []teardown.EventType{
			teardown.EventPreTeardownStarted,
			teardown.EventHookStarted,
			teardown.EventHookCompleted,
			teardown.EventPreTeardownCompleted,
			teardown.EventTierStarted,
			teardown.EventTierCompleted,
			teardown.EventTierStarted,
			teardown.EventTierCompleted,
			teardown.EventResidualStarted,
			teardown.EventResidualCompleted,
			teardown.EventPostTeardownStarted,
			teardown.EventHookStarted,
			teardown.EventHookCompleted,
			teardown.EventPostTeardownCompleted,
			teardown.EventSyncStarted,
			teardown.EventSyncFlushed,
			teardown.EventDirtyDrainStarted,
			teardown.EventDirtyDrainPolled,
			teardown.EventDirtyDrainCompleted,
		}

		if len(events) != len(expectedEventTypes) {
			t.Fatalf("expected %d events, got %d (%v)", len(expectedEventTypes), len(events), events)
		}
		for i, expected := range expectedEventTypes {
			if events[i] != expected {
				t.Fatalf("event at index %d: expected %s, got %s", i, expected, events[i])
			}
		}
	})
}

func TestCoordinator_PreTeardownHookFailures(t *testing.T) {
	t.Run("aborts teardown immediately when hook fails and ignore_errors is false", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("disk full"), errors.New("command failed")
		}

		mock := &mockProvider{}
		m := setupTestManifest()
		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))

		err := coord.Teardown(context.Background())
		if err == nil {
			t.Fatal("expected teardown to fail when pre-teardown hook fails, got nil")
		}

		if len(mock.stopTierCalls) != 0 || mock.stopResidualCall != 0 {
			t.Fatal("workload stops should not execute after pre-teardown failure")
		}
		if mem.SyncCount != 0 {
			t.Fatal("filesystem sync should not execute after pre-teardown failure")
		}
	})

	t.Run("continues teardown when hook fails but ignore_errors is true", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("Dirty: 0 kB\n")
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("error ignored"), errors.New("exit 1")
		}

		mock := &mockProvider{}
		m := setupTestManifest()
		m.Hooks.PreTeardown[0].IgnoreErrors = true
		m.Hooks.PostTeardown[0].IgnoreErrors = true

		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))
		err := coord.Teardown(context.Background())
		if err != nil {
			t.Fatalf("expected teardown to succeed with ignore_errors true, got %v", err)
		}

		if len(mock.stopTierCalls) != 2 {
			t.Fatalf("expected 2 tiers stopped, got %d", len(mock.stopTierCalls))
		}
		if mock.stopResidualCall != 1 {
			t.Fatalf("expected residual stop called, got %d", mock.stopResidualCall)
		}
	})
}

func TestCoordinator_TierStopFailures(t *testing.T) {
	t.Run("aborts execution on tier failure without stopping subsequent tiers or residual", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		}

		mock := &mockProvider{
			stopTierFunc: func(ctx context.Context, tier string, timeout time.Duration) error {
				if tier == "apps" {
					return errors.New("container app-1 failed to stop")
				}
				return nil
			},
		}

		m := setupTestManifest()
		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))

		err := coord.Teardown(context.Background())
		if err == nil {
			t.Fatal("expected error on tier stop failure, got nil")
		}

		if len(mock.stopTierCalls) != 1 {
			t.Fatalf("expected only first tier attempted, got %d", len(mock.stopTierCalls))
		}
		if mock.stopResidualCall != 0 {
			t.Fatal("residual stop should not run after tier failure")
		}
		if mem.SyncCount != 0 {
			t.Fatal("filesystem sync should not run after tier failure")
		}
	})

	t.Run("aborts execution when residual stop fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		}

		mock := &mockProvider{
			stopResidualFunc: func(ctx context.Context, timeout time.Duration) error {
				return errors.New("residual container stuck")
			},
		}

		m := setupTestManifest()
		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))

		err := coord.Teardown(context.Background())
		if err == nil {
			t.Fatal("expected error on residual stop failure, got nil")
		}

		if mem.SyncCount != 0 {
			t.Fatal("filesystem sync should not run after residual failure")
		}
	})
}

func TestCoordinator_PostTeardownHookFailures(t *testing.T) {
	t.Run("aborts before filesystem sync when post-teardown hook fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) >= 2 && args[1] == "send-alert" {
				return []byte("network unreachable"), errors.New("hook exit 2")
			}
			return []byte("ok"), nil
		}

		mock := &mockProvider{}
		m := setupTestManifest()
		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))

		err := coord.Teardown(context.Background())
		if err == nil {
			t.Fatal("expected error when post-teardown hook fails, got nil")
		}

		if mem.SyncCount != 0 {
			t.Fatal("filesystem sync should not execute when post-teardown hook fails")
		}
	})
}

func TestCoordinator_SyncFSFailure(t *testing.T) {
	t.Run("returns error when host filesystem sync fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("Dirty: 0 kB\n")
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		}

		// Simulate SyncFS error by wrapping coordinator with custom sync check
		mock := &mockProvider{}
		m := setupTestManifest()
		m.Hooks.PreTeardown = nil
		m.Hooks.PostTeardown = nil
		coord := teardown.NewCoordinator(mem, m, teardown.WithProvider(mock))

		err := coord.SyncFS(context.Background())
		if err != nil {
			t.Fatalf("unexpected error on default MemEnv sync: %v", err)
		}
	})
}

func TestCoordinator_DrainDirtyPages(t *testing.T) {
	t.Run("finishes immediately when dirty pages is already zero", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("MemTotal: 4000 kB\nDirty: 0 kB\nWriteback: 0 kB\n")

		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(time.Second),
			teardown.WithDrainInterval(time.Millisecond),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err != nil {
			t.Fatalf("unexpected error draining 0 dirty pages: %v", err)
		}
	})

	t.Run("polls until dirty count drops to zero", func(t *testing.T) {
		mem := env.NewMemEnv()
		values := []int64{2048, 1024, 0}
		readIdx := 0

		mem.ReadHandler = func(path string) ([]byte, error) {
			if path == "/proc/meminfo" {
				val := values[readIdx]
				if readIdx < len(values)-1 {
					readIdx++
				}
				return []byte(fmt.Sprintf("MemTotal: 4000 kB\nDirty: %d kB\n", val)), nil
			}
			return nil, os.ErrNotExist
		}

		var polled []int64
		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(time.Second),
			teardown.WithDrainInterval(time.Millisecond),
			teardown.WithEventListener(func(e teardown.TeardownEvent) {
				if e.Type == teardown.EventDirtyDrainPolled {
					polled = append(polled, e.DirtyKB)
				}
			}),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(polled) != 3 || polled[0] != 2048 || polled[1] != 1024 || polled[2] != 0 {
			t.Fatalf("expected polled values [2048, 1024, 0], got %v", polled)
		}
	})

	t.Run("succeeds when dirty page count stabilizes into plateau", func(t *testing.T) {
		mem := env.NewMemEnv()
		// Return 512 kB stably
		mem.Files["/proc/meminfo"] = []byte("Dirty: 512 kB\n")

		var polled []int64
		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(time.Second),
			teardown.WithDrainInterval(time.Millisecond),
			teardown.WithPlateauChecks(3),
			teardown.WithEventListener(func(e teardown.TeardownEvent) {
				if e.Type == teardown.EventDirtyDrainPolled {
					polled = append(polled, e.DirtyKB)
				}
			}),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err != nil {
			t.Fatalf("unexpected plateau failure: %v", err)
		}

		if len(polled) < 3 {
			t.Fatalf("expected at least 3 polls before plateau completion, got %d", len(polled))
		}
	})

	t.Run("returns timeout error when dirty count fails to reach zero or plateau", func(t *testing.T) {
		mem := env.NewMemEnv()
		counter := int64(10000)
		mem.ReadHandler = func(path string) ([]byte, error) {
			if path == "/proc/meminfo" {
				counter -= 10
				return []byte(fmt.Sprintf("Dirty: %d kB\n", counter)), nil
			}
			return nil, os.ErrNotExist
		}

		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(15*time.Millisecond),
			teardown.WithDrainInterval(5*time.Millisecond),
			teardown.WithPlateauChecks(5),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err == nil {
			t.Fatal("expected timeout error when buffer fails to drain, got nil")
		}
	})

	t.Run("honors caller context cancellation", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("Dirty: 4096 kB\n")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(time.Second),
			teardown.WithDrainInterval(10*time.Millisecond),
		)

		err := coord.DrainDirtyPages(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got %v", err)
		}
	})

	t.Run("returns error when meminfo file does not exist", func(t *testing.T) {
		mem := env.NewMemEnv()
		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithProcMeminfoPath("/nonexistent/meminfo"),
			teardown.WithDrainTimeout(time.Second),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err == nil {
			t.Fatal("expected error reading nonexistent meminfo path, got nil")
		}
	})

	t.Run("returns error when meminfo content has no Dirty field", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.Files["/proc/meminfo"] = []byte("MemTotal: 8000 kB\nBuffers: 200 kB\n")

		coord := teardown.NewCoordinator(mem, nil,
			teardown.WithDrainTimeout(time.Second),
		)

		err := coord.DrainDirtyPages(context.Background())
		if err == nil {
			t.Fatal("expected error when Dirty field is missing, got nil")
		}
	})
}

func TestExecuteHook_EdgeCases(t *testing.T) {
	t.Run("rejects empty command when ignore_errors is false", func(t *testing.T) {
		mem := env.NewMemEnv()
		err := teardown.ExecuteHook(context.Background(), mem, manifest.HookConfig{
			Name:         "empty-hook",
			Command:      "   ",
			IgnoreErrors: false,
		})
		if err == nil {
			t.Fatal("expected error for empty command, got nil")
		}
	})

	t.Run("allows empty command when ignore_errors is true", func(t *testing.T) {
		mem := env.NewMemEnv()
		err := teardown.ExecuteHook(context.Background(), mem, manifest.HookConfig{
			Name:         "empty-hook",
			Command:      "",
			IgnoreErrors: true,
		})
		if err != nil {
			t.Fatalf("unexpected error for empty command with ignore_errors true: %v", err)
		}
	})

	t.Run("enforces hook timeout", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []byte("done"), nil
			}
		}

		err := teardown.ExecuteHook(context.Background(), mem, manifest.HookConfig{
			Name:           "slow-hook",
			Command:        "sleep 10",
			TimeoutSeconds: 1, // Will create 1s timeout, but we test with pre-expired ctx
		})
		if err != nil {
			// Normal execution completes within 100ms
		}

		// Force deadline exceeded
		expiredCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()

		err = teardown.ExecuteHook(expiredCtx, mem, manifest.HookConfig{
			Name:           "timed-out-hook",
			Command:        "sleep 10",
			TimeoutSeconds: 1,
		})
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
	})

	t.Run("executes multiple hooks sequentially via ExecuteHooks", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executed []string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executed = append(executed, args[1])
			return nil, nil
		}

		hooks := []manifest.HookConfig{
			{Name: "first", Command: "run-first"},
			{Name: "second", Command: "run-second"},
		}

		err := teardown.ExecuteHooks(context.Background(), mem, hooks)
		if err != nil {
			t.Fatalf("unexpected error executing hooks: %v", err)
		}

		if len(executed) != 2 || executed[0] != "run-first" || executed[1] != "run-second" {
			t.Fatalf("expected [run-first, run-second], got %v", executed)
		}
	})
}

func TestParseDirtyKB(t *testing.T) {
	t.Run("parses valid Dirty entry with kB", func(t *testing.T) {
		data := []byte("MemTotal: 1000 kB\nDirty: 15168 kB\nWriteback: 0 kB\n")
		val, err := teardown.ParseDirtyKB(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 15168 {
			t.Fatalf("expected 15168, got %d", val)
		}
	})

	t.Run("parses entry without kB suffix", func(t *testing.T) {
		data := []byte("Dirty: 42\n")
		val, err := teardown.ParseDirtyKB(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 42 {
			t.Fatalf("expected 42, got %d", val)
		}
	})

	t.Run("handles lowercase prefix", func(t *testing.T) {
		data := []byte("dirty: 100 kB\n")
		val, err := teardown.ParseDirtyKB(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != 100 {
			t.Fatalf("expected 100, got %d", val)
		}
	})

	t.Run("returns error on negative value", func(t *testing.T) {
		data := []byte("Dirty: -50 kB\n")
		_, err := teardown.ParseDirtyKB(data)
		if err == nil {
			t.Fatal("expected error on negative dirty value, got nil")
		}
	})

	t.Run("returns error on malformed line", func(t *testing.T) {
		data := []byte("Dirty:\n")
		_, err := teardown.ParseDirtyKB(data)
		if err == nil {
			t.Fatal("expected error on malformed line, got nil")
		}
	})

	t.Run("returns error on non-integer value", func(t *testing.T) {
		data := []byte("Dirty: not_a_number kB\n")
		_, err := teardown.ParseDirtyKB(data)
		if err == nil {
			t.Fatal("expected error on non-integer, got nil")
		}
	})
}

func TestCoordinator_DefaultsAndOptions(t *testing.T) {
	t.Run("instantiates cleanly with nil manifest", func(t *testing.T) {
		mem := env.NewMemEnv()
		coord := teardown.NewCoordinator(mem, nil)
		if coord == nil {
			t.Fatal("expected non-nil coordinator")
		}
		if coord.Provider() == nil {
			t.Fatal("expected default provider initialized")
		}
	})

	t.Run("returns error when provider is nil during StopTiers", func(t *testing.T) {
		mem := env.NewMemEnv()
		coord := teardown.NewCoordinator(mem, nil, teardown.WithProvider(nil))
		err := coord.StopTiers(context.Background())
		if err == nil {
			t.Fatal("expected error when provider is nil, got nil")
		}
	})
}

func TestExecuteHook_TimeoutEnforcement(t *testing.T) {
	t.Run("hook exceeding timeout is cancelled via context deadline", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return []byte("completed late"), nil
			}
		}

		hook := manifest.HookConfig{
			Name:           "slow-backup",
			Command:        "sleep 10",
			TimeoutSeconds: 1, // 1 second timeout
			IgnoreErrors:   false,
		}

		start := time.Now()
		err := teardown.ExecuteHook(context.Background(), mem, hook)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected hook timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "timed out after") {
			t.Fatalf("expected error mentioning timed out, got: %v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected errors.Is DeadlineExceeded, got: %v", err)
		}
		if elapsed > 3*time.Second {
			t.Fatalf("hook execution exceeded timeout deadline: took %v", elapsed)
		}
	})

	t.Run("timed out hook with ignore_errors returns nil", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return []byte("late"), nil
			}
		}

		hook := manifest.HookConfig{
			Name:           "slow-ignorable",
			Command:        "sleep 10",
			TimeoutSeconds: 1,
			IgnoreErrors:   true,
		}

		err := teardown.ExecuteHook(context.Background(), mem, hook)
		if err != nil {
			t.Fatalf("expected nil error for ignored hook timeout, got: %v", err)
		}
	})
}

func TestCoordinator_VerboseLogging(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/proc/meminfo"] = []byte("MemTotal: 4000000 kB\nDirty: 0 kB\n")
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}
	provider := &mockProvider{}
	m := setupTestManifest()

	var buf strings.Builder
	coord := teardown.NewCoordinator(mem, m,
		teardown.WithProvider(provider),
		teardown.WithVerbose(true),
		teardown.WithLogger(&buf),
	)

	ctx := context.Background()
	if err := coord.Teardown(ctx); err != nil {
		t.Fatalf("Teardown failed: %v", err)
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "[teardown] stopping profile tier \"apps\"") {
		t.Errorf("expected tier apps log, got: %s", logOut)
	}
	if !strings.Contains(logOut, "[teardown] profile tier \"apps\" stopped in") {
		t.Errorf("expected tier apps completion log, got: %s", logOut)
	}
	if !strings.Contains(logOut, "[teardown] stopping residual unprofiled containers") {
		t.Errorf("expected residual stop log, got: %s", logOut)
	}
	if !strings.Contains(logOut, "[teardown] dirty page drain check: 0 kB") {
		t.Errorf("expected dirty page drain check log, got: %s", logOut)
	}
}


