package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/rtc"
	"github.com/adrianfinantyo/arkon/internal/state"
	"github.com/adrianfinantyo/arkon/internal/teardown"
)

// EventType identifies lifecycle transitions emitted by the coordinator.
type EventType string

const (
	EventInvariantsChecked      EventType = "invariants_checked"
	EventPreSleepHooksStarted   EventType = "pre_sleep_hooks_started"
	EventPreSleepHookStarted    EventType = "pre_sleep_hook_started"
	EventPreSleepHookCompleted  EventType = "pre_sleep_hook_completed"
	EventPreSleepHooksCompleted EventType = "pre_sleep_hooks_completed"
	EventTeardownStarted        EventType = "teardown_started"
	EventTeardownCompleted      EventType = "teardown_completed"
	EventRTCArmed               EventType = "rtc_armed"
	EventPowerActionStarted     EventType = "power_action_started"
	EventPowerActionCompleted   EventType = "power_action_completed"
	EventPostWakeHooksStarted   EventType = "post_wake_hooks_started"
	EventPostWakeHookStarted    EventType = "post_wake_hook_started"
	EventPostWakeHookCompleted  EventType = "post_wake_hook_completed"
	EventPostWakeHooksCompleted EventType = "post_wake_hooks_completed"
	EventDryRunCompleted        EventType = "dry_run_completed"
)

// Event captures progress updates for callers and structured logs.
type Event struct {
	Type    EventType      `json:"type"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	Err     error          `json:"-"`
}

// InhibitionError indicates sleep was blocked by daytime protection or active locks.
type InhibitionError struct {
	Result state.SleepCheckResult
}

func (e *InhibitionError) Error() string {
	return e.Result.Reason
}

// SleepResult represents the outcome of a sleep operation.
type SleepResult struct {
	Status          string    `json:"status"`
	DryRun          bool      `json:"dry_run,omitempty"`
	Forced          bool      `json:"forced,omitempty"`
	TiersStopped    []string  `json:"tiers_stopped"`
	ResidualStopped bool      `json:"residual_stopped"`
	DiskSynced      bool      `json:"disk_synced"`
	WakeAlarmArmed  bool      `json:"wakealarm_armed"`
	NextWakeTime    time.Time `json:"next_wake_time,omitempty"`
	WakeDeltaSec    int64     `json:"wake_delta_seconds,omitempty"`
	PowerAction     string    `json:"power_action"`
}

// Coordinator orchestrates daytime checks, locks, hooks, teardown, RTC, and poweroff.
type Coordinator struct {
	env           env.HostEnvironment
	manifest      *manifest.Manifest
	stateMgr      *state.Manager
	rtcMgr        *rtc.Manager
	teardownCoord *teardown.Coordinator
	force         bool
	dryRun        bool
	clock         func() time.Time
	eventListener func(Event)
}

// CoordinatorOption customizes coordinator behavior.
type CoordinatorOption func(*Coordinator)

// WithForce overrides daytime protection and active inhibition locks.
func WithForce(force bool) CoordinatorOption {
	return func(c *Coordinator) {
		c.force = force
	}
}

// WithDryRun simulates the sleep sequence without stopping workloads, writing sysfs, or cutting power.
func WithDryRun(dryRun bool) CoordinatorOption {
	return func(c *Coordinator) {
		c.dryRun = dryRun
	}
}

// WithStateManager injects a custom state manager.
func WithStateManager(mgr *state.Manager) CoordinatorOption {
	return func(c *Coordinator) {
		c.stateMgr = mgr
	}
}

// WithRTCManager injects a custom RTC manager.
func WithRTCManager(mgr *rtc.Manager) CoordinatorOption {
	return func(c *Coordinator) {
		c.rtcMgr = mgr
	}
}

// WithTeardownCoordinator injects a custom workload teardown coordinator.
func WithTeardownCoordinator(coord *teardown.Coordinator) CoordinatorOption {
	return func(c *Coordinator) {
		c.teardownCoord = coord
	}
}

// WithClock overrides the clock function for deterministic time testing.
func WithClock(clock func() time.Time) CoordinatorOption {
	return func(c *Coordinator) {
		c.clock = clock
	}
}

// WithEventListener attaches a callback receiving granular lifecycle progress events.
func WithEventListener(listener func(Event)) CoordinatorOption {
	return func(c *Coordinator) {
		c.eventListener = listener
	}
}

// NewCoordinator constructs a coordinator configured from host environment and manifest settings.
func NewCoordinator(environment env.HostEnvironment, m *manifest.Manifest, opts ...CoordinatorOption) *Coordinator {
	if m == nil {
		m = &manifest.Manifest{}
	}
	m.ApplyDefaults()
	if m.Host.Timezone == "" {
		m.Host.Timezone = "UTC"
	}
	if len(m.Teardown.ProfileOrder) == 0 {
		m.Teardown.ProfileOrder = []string{"apps", "infra"}
	}

	c := &Coordinator{
		env:      environment,
		manifest: m,
		clock:    environment.Now,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.stateMgr == nil {
		c.stateMgr = state.NewManager("", state.WithClock(c.clock))
	}
	if c.rtcMgr == nil {
		var rtcOpts []rtc.Option
		if m.Host.RTCDevice != "" {
			rtcOpts = append(rtcOpts, rtc.WithSysfsPath(m.Host.RTCDevice))
		}
		if m.Host.Timezone != "" {
			if loc, err := time.LoadLocation(m.Host.Timezone); err == nil {
				rtcOpts = append(rtcOpts, rtc.WithLocation(loc))
			}
		}
		c.rtcMgr = rtc.NewManager(environment, rtcOpts...)
	}
	if c.teardownCoord == nil {
		c.teardownCoord = teardown.NewCoordinator(environment, m)
	}

	return c
}

// Execute orchestrates the full sleep lifecycle sequence in strict order.
func (c *Coordinator) Execute(ctx context.Context) (*SleepResult, error) {
	now := c.clock()

	loc := time.Local
	if c.manifest.Host.Timezone != "" {
		if l, err := time.LoadLocation(c.manifest.Host.Timezone); err == nil {
			loc = l
		}
	}
	nowInLoc := now.In(loc)

	// 1. Evaluate daytime operational window and active locks
	daytimeProtected := true
	if c.manifest.Inhibition.DaytimeProtection != nil {
		daytimeProtected = *c.manifest.Inhibition.DaytimeProtection
	}

	daytimeStart := c.manifest.Inhibition.DaytimeStart
	if daytimeStart == "" {
		daytimeStart = c.manifest.Circadian.WakeAt
	}
	daytimeEnd := c.manifest.Inhibition.DaytimeEnd
	if daytimeEnd == "" {
		daytimeEnd = c.manifest.Circadian.SleepAt
	}

	daytimeCfg := state.DaytimeConfig{
		WakeAt:            c.manifest.Circadian.WakeAt,
		SleepAt:           c.manifest.Circadian.SleepAt,
		DaytimeStart:      daytimeStart,
		DaytimeEnd:        daytimeEnd,
		DaytimeProtection: daytimeProtected,
	}

	activeLocks, err := c.stateMgr.ListLocks()
	if err != nil {
		return nil, fmt.Errorf("failed to list inhibition locks: %w", err)
	}

	eval := state.EvaluateSleepAllowed(nowInLoc, daytimeCfg, activeLocks, c.force)
	if eval.Inhibited {
		return nil, &InhibitionError{Result: eval}
	}

	c.emit(Event{
		Type:    EventInvariantsChecked,
		Message: "Invariants confirmed (daytime clear, active locks cleared)",
	})

	// 2. Compute next scheduled wake epoch from circadian policy
	nextWake, err := CalculateNextWake(now, c.manifest.Circadian, c.manifest.Host.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate next wake time: %w", err)
	}

	deltaSec := int64(nextWake.Sub(now).Seconds())
	if err := c.rtcMgr.ValidateSafetyBuffer(deltaSec); err != nil {
		return nil, fmt.Errorf("circadian safety buffer violation: %w", err)
	}

	// 3. Dry run mode
	if c.dryRun {
		c.emit(Event{
			Type:    EventDryRunCompleted,
			Message: "Dry run completed cleanly",
			Data: map[string]any{
				"tiers_stopped":      c.manifest.Teardown.ProfileOrder,
				"next_wake_time":     nextWake,
				"wake_delta_seconds": deltaSec,
				"power_action":       c.manifest.Host.PowerAction,
			},
		})
		return &SleepResult{
			Status:          "completed",
			DryRun:          true,
			Forced:          c.force,
			TiersStopped:    c.manifest.Teardown.ProfileOrder,
			ResidualStopped: true,
			DiskSynced:      true,
			WakeAlarmArmed:  true,
			NextWakeTime:    nextWake,
			WakeDeltaSec:    deltaSec,
			PowerAction:     c.manifest.Host.PowerAction,
		}, nil
	}

	// 4. Workload teardown pipeline
	c.emit(Event{Type: EventTeardownStarted})
	if err := c.teardownCoord.Teardown(ctx); err != nil {
		return nil, fmt.Errorf("workload teardown pipeline failed: %w", err)
	}
	c.emit(Event{Type: EventTeardownCompleted})

	// 5. Pre-sleep hooks (execute AFTER workload teardown and filesystem buffer drain)
	if len(c.manifest.Hooks.PreSleep) > 0 {
		c.emit(Event{Type: EventPreSleepHooksStarted})
		for _, hook := range c.manifest.Hooks.PreSleep {
			c.emit(Event{Type: EventPreSleepHookStarted, Message: hook.Name})
			if err := teardown.ExecuteHook(ctx, c.env, hook); err != nil {
				return nil, fmt.Errorf("pre-sleep hook %q failed: %w", hook.Name, err)
			}
			c.emit(Event{Type: EventPreSleepHookCompleted, Message: hook.Name})
		}
		c.emit(Event{Type: EventPreSleepHooksCompleted})
	}

	// 6. Hardware RTC wakealarm programming
	if err := c.rtcMgr.SetWakeAlarm(deltaSec); err != nil {
		return nil, fmt.Errorf("failed to program hardware RTC wakealarm: %w", err)
	}
	c.emit(Event{
		Type:    EventRTCArmed,
		Message: fmt.Sprintf("Hardware wakealarm armed for %s (+%ds)", nextWake.Format(time.RFC3339), deltaSec),
	})

	// 7. Host power transition
	c.emit(Event{
		Type:    EventPowerActionStarted,
		Message: fmt.Sprintf("Initiating host power transition: systemctl %s", c.manifest.Host.PowerAction),
	})
	if err := c.rtcMgr.ExecutePowerAction(ctx, c.manifest.Host.PowerAction); err != nil {
		return nil, fmt.Errorf("host power transition failed: %w", err)
	}
	c.emit(Event{Type: EventPowerActionCompleted})

	// 8. Execute post-wake hooks when resuming from suspend or hibernate
	action := strings.ToLower(c.manifest.Host.PowerAction)
	if action == "suspend" || action == "hibernate" {
		if err := c.ExecutePostWake(ctx); err != nil {
			return nil, fmt.Errorf("post-wake hooks failed: %w", err)
		}
	}

	return &SleepResult{
		Status:          "completed",
		DryRun:          false,
		Forced:          c.force,
		TiersStopped:    c.manifest.Teardown.ProfileOrder,
		ResidualStopped: true,
		DiskSynced:      true,
		WakeAlarmArmed:  true,
		NextWakeTime:    nextWake,
		WakeDeltaSec:    deltaSec,
		PowerAction:     c.manifest.Host.PowerAction,
	}, nil
}

// ExecutePostWake runs all configured post-wake hooks.
func (c *Coordinator) ExecutePostWake(ctx context.Context) error {
	if len(c.manifest.Hooks.PostWake) == 0 {
		return nil
	}
	c.emit(Event{Type: EventPostWakeHooksStarted})
	for _, hook := range c.manifest.Hooks.PostWake {
		c.emit(Event{Type: EventPostWakeHookStarted, Message: hook.Name})
		if err := teardown.ExecuteHook(ctx, c.env, hook); err != nil {
			return fmt.Errorf("post-wake hook %q failed: %w", hook.Name, err)
		}
		c.emit(Event{Type: EventPostWakeHookCompleted, Message: hook.Name})
	}
	c.emit(Event{Type: EventPostWakeHooksCompleted})
	return nil
}

func (c *Coordinator) emit(event Event) {
	if c.eventListener != nil {
		c.eventListener(event)
	}
}
