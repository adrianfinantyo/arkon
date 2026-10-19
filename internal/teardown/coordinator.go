package teardown

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

// Default filesystem and timing parameters for dirty page cache flushing.
const (
	DefaultProcMeminfoPath = "/proc/meminfo"
	DefaultDrainTimeout    = 15 * time.Second
	DefaultDrainInterval   = 100 * time.Millisecond
	DefaultPlateauChecks   = 3
)

// EventType identifies specific lifecycle transitions emitted by the coordinator.
type EventType string

const (
	EventPreTeardownStarted    EventType = "pre_teardown_started"
	EventPreTeardownCompleted  EventType = "pre_teardown_completed"
	EventTierStarted           EventType = "tier_started"
	EventTierCompleted         EventType = "tier_completed"
	EventTierFailed            EventType = "tier_failed"
	EventResidualStarted       EventType = "residual_started"
	EventResidualCompleted     EventType = "residual_completed"
	EventResidualFailed        EventType = "residual_failed"
	EventPostTeardownStarted   EventType = "post_teardown_started"
	EventPostTeardownCompleted EventType = "post_teardown_completed"
	EventHookStarted           EventType = "hook_started"
	EventHookCompleted         EventType = "hook_completed"
	EventHookFailed            EventType = "hook_failed"
	EventSyncStarted           EventType = "sync_started"
	EventSyncFlushed           EventType = "sync_flushed"
	EventSyncFailed            EventType = "sync_failed"
	EventDirtyDrainStarted     EventType = "dirty_drain_started"
	EventDirtyDrainPolled      EventType = "dirty_drain_polled"
	EventDirtyDrainCompleted   EventType = "dirty_drain_completed"
	EventDirtyDrainFailed      EventType = "dirty_drain_failed"
)

// TeardownEvent captures progress updates for interactive terminals and structured logs.
type TeardownEvent struct {
	Type     EventType     `json:"type"`
	Tier     string        `json:"tier,omitempty"`
	Hook     string        `json:"hook,omitempty"`
	DirtyKB  int64         `json:"dirty_kb,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Err      error         `json:"-"`
}

// Coordinator orchestrates phased workload stops, lifecycle hooks, and disk page cache flushes.
type Coordinator struct {
	env             env.HostEnvironment
	provider        TeardownProvider
	manifest        *manifest.Manifest
	procMeminfoPath string
	drainTimeout    time.Duration
	drainInterval   time.Duration
	plateauChecks   int
	eventListener   func(TeardownEvent)
}

// CoordinatorOption customizes coordinator behavior.
type CoordinatorOption func(*Coordinator)

// WithProvider overrides the default workload teardown provider.
func WithProvider(p TeardownProvider) CoordinatorOption {
	return func(c *Coordinator) {
		c.provider = p
	}
}

// WithProcMeminfoPath overrides the path to the kernel meminfo interface.
func WithProcMeminfoPath(path string) CoordinatorOption {
	return func(c *Coordinator) {
		c.procMeminfoPath = path
	}
}

// WithDrainTimeout sets the maximum duration to wait for kernel write buffers to drain.
func WithDrainTimeout(d time.Duration) CoordinatorOption {
	return func(c *Coordinator) {
		c.drainTimeout = d
	}
}

// WithDrainInterval sets the polling cadence for inspecting dirty page counters.
func WithDrainInterval(d time.Duration) CoordinatorOption {
	return func(c *Coordinator) {
		c.drainInterval = d
	}
}

// WithPlateauChecks sets the consecutive stable count required to recognize a buffer plateau.
func WithPlateauChecks(checks int) CoordinatorOption {
	return func(c *Coordinator) {
		c.plateauChecks = checks
	}
}

// WithEventListener attaches a callback receiving granular lifecycle progress events.
func WithEventListener(listener func(TeardownEvent)) CoordinatorOption {
	return func(c *Coordinator) {
		c.eventListener = listener
	}
}

// NewCoordinator constructs a coordinator configured from host environment and manifest settings.
func NewCoordinator(environment env.HostEnvironment, m *manifest.Manifest, opts ...CoordinatorOption) *Coordinator {
	if m == nil {
		m = &manifest.Manifest{}
		m.ApplyDefaults()
	}

	c := &Coordinator{
		env:             environment,
		provider:        NewDockerComposeProviderFromManifest(environment, m.Teardown),
		manifest:        m,
		procMeminfoPath: DefaultProcMeminfoPath,
		drainTimeout:    DefaultDrainTimeout,
		drainInterval:   DefaultDrainInterval,
		plateauChecks:   DefaultPlateauChecks,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Provider returns the active workload teardown provider.
func (c *Coordinator) Provider() TeardownProvider {
	return c.provider
}

// Teardown executes all teardown stages in strict sequential dependency order.
func (c *Coordinator) Teardown(ctx context.Context) error {
	if err := c.ExecutePreTeardownHooks(ctx); err != nil {
		return fmt.Errorf("pre-teardown hooks failed: %w", err)
	}

	if err := c.StopWorkloads(ctx); err != nil {
		return fmt.Errorf("workload teardown failed: %w", err)
	}

	if err := c.ExecutePostTeardownHooks(ctx); err != nil {
		return fmt.Errorf("post-teardown hooks failed: %w", err)
	}

	if err := c.SyncAndDrain(ctx); err != nil {
		return fmt.Errorf("filesystem sync and buffer drain failed: %w", err)
	}

	return nil
}

// ExecutePreTeardownHooks runs configured pre-teardown commands in sequence.
func (c *Coordinator) ExecutePreTeardownHooks(ctx context.Context) error {
	hooks := c.manifest.Hooks.PreTeardown
	if len(hooks) == 0 {
		return nil
	}

	c.emit(TeardownEvent{Type: EventPreTeardownStarted})
	for _, hook := range hooks {
		c.emit(TeardownEvent{Type: EventHookStarted, Hook: hook.Name})
		if err := ExecuteHook(ctx, c.env, hook); err != nil {
			c.emit(TeardownEvent{Type: EventHookFailed, Hook: hook.Name, Err: err})
			return err
		}
		c.emit(TeardownEvent{Type: EventHookCompleted, Hook: hook.Name})
	}
	c.emit(TeardownEvent{Type: EventPreTeardownCompleted})
	return nil
}

// StopWorkloads runs phased profile stopping followed by the residual untagged container catch-all.
func (c *Coordinator) StopWorkloads(ctx context.Context) error {
	if err := c.StopTiers(ctx); err != nil {
		return err
	}
	return c.StopResidual(ctx)
}

// StopTiers stops profile tiers sequentially according to configured profile order.
func (c *Coordinator) StopTiers(ctx context.Context) error {
	if c.provider == nil {
		return errors.New("no teardown provider configured")
	}

	timeout := c.tierTimeout()
	for _, tier := range c.manifest.Teardown.ProfileOrder {
		trimmed := strings.TrimSpace(tier)
		if trimmed == "" {
			continue
		}
		c.emit(TeardownEvent{Type: EventTierStarted, Tier: trimmed})
		if err := c.provider.StopTier(ctx, trimmed, timeout); err != nil {
			c.emit(TeardownEvent{Type: EventTierFailed, Tier: trimmed, Err: err})
			return fmt.Errorf("failed to stop tier %q: %w", trimmed, err)
		}
		c.emit(TeardownEvent{Type: EventTierCompleted, Tier: trimmed})
	}

	return nil
}

// StopResidual stops remaining unprofiled containers across configured stacks.
func (c *Coordinator) StopResidual(ctx context.Context) error {
	if c.provider == nil {
		return errors.New("no teardown provider configured")
	}

	timeout := c.tierTimeout()
	c.emit(TeardownEvent{Type: EventResidualStarted})
	if err := c.provider.StopResidual(ctx, timeout); err != nil {
		c.emit(TeardownEvent{Type: EventResidualFailed, Err: err})
		return fmt.Errorf("failed to stop residual workloads: %w", err)
	}
	c.emit(TeardownEvent{Type: EventResidualCompleted})
	return nil
}

// ExecutePostTeardownHooks runs configured post-teardown commands in sequence.
func (c *Coordinator) ExecutePostTeardownHooks(ctx context.Context) error {
	hooks := c.manifest.Hooks.PostTeardown
	if len(hooks) == 0 {
		return nil
	}

	c.emit(TeardownEvent{Type: EventPostTeardownStarted})
	for _, hook := range hooks {
		c.emit(TeardownEvent{Type: EventHookStarted, Hook: hook.Name})
		if err := ExecuteHook(ctx, c.env, hook); err != nil {
			c.emit(TeardownEvent{Type: EventHookFailed, Hook: hook.Name, Err: err})
			return err
		}
		c.emit(TeardownEvent{Type: EventHookCompleted, Hook: hook.Name})
	}
	c.emit(TeardownEvent{Type: EventPostTeardownCompleted})
	return nil
}

// SyncFS instructs the host kernel to initiate flushing dirty file pages to physical media.
func (c *Coordinator) SyncFS(ctx context.Context) error {
	c.emit(TeardownEvent{Type: EventSyncStarted})
	if err := c.env.SyncFS(); err != nil {
		c.emit(TeardownEvent{Type: EventSyncFailed, Err: err})
		return fmt.Errorf("failed to sync filesystem: %w", err)
	}
	c.emit(TeardownEvent{Type: EventSyncFlushed})
	return nil
}

// SyncAndDrain calls SyncFS and monitors kernel dirty bytes until drained or plateaued.
func (c *Coordinator) SyncAndDrain(ctx context.Context) error {
	if err := c.SyncFS(ctx); err != nil {
		return err
	}
	return c.DrainDirtyPages(ctx)
}

// DrainDirtyPages polls the kernel dirty page counter until it drops to zero or stabilizes.
func (c *Coordinator) DrainDirtyPages(ctx context.Context) error {
	c.emit(TeardownEvent{Type: EventDirtyDrainStarted})

	timeout := c.drainTimeout
	if timeout <= 0 {
		timeout = DefaultDrainTimeout
	}

	interval := c.drainInterval
	if interval <= 0 {
		interval = DefaultDrainInterval
	}

	requiredPlateau := c.plateauChecks
	if requiredPlateau < 2 {
		requiredPlateau = DefaultPlateauChecks
	}

	drainCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		lastDirty    int64 = -1
		plateauCount int   = 0
	)

	for {
		data, err := c.env.ReadFile(c.procMeminfoPath)
		if err != nil {
			c.emit(TeardownEvent{Type: EventDirtyDrainFailed, Err: err})
			return fmt.Errorf("failed to read %s: %w", c.procMeminfoPath, err)
		}

		dirty, err := ParseDirtyKB(data)
		if err != nil {
			c.emit(TeardownEvent{Type: EventDirtyDrainFailed, Err: err})
			return fmt.Errorf("failed to parse dirty page count: %w", err)
		}

		c.emit(TeardownEvent{Type: EventDirtyDrainPolled, DirtyKB: dirty})

		if dirty <= 0 {
			c.emit(TeardownEvent{Type: EventDirtyDrainCompleted, DirtyKB: 0})
			return nil
		}

		if lastDirty >= 0 && dirty == lastDirty {
			plateauCount++
			if plateauCount >= requiredPlateau {
				c.emit(TeardownEvent{Type: EventDirtyDrainCompleted, DirtyKB: dirty})
				return nil
			}
		} else {
			lastDirty = dirty
			plateauCount = 1
		}

		select {
		case <-drainCtx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			err := fmt.Errorf("dirty page cache failed to drain within %v (last dirty: %d kB)", timeout, dirty)
			c.emit(TeardownEvent{Type: EventDirtyDrainFailed, DirtyKB: dirty, Err: err})
			return err
		case <-time.After(interval):
		}
	}
}

func (c *Coordinator) tierTimeout() time.Duration {
	if c.manifest != nil && c.manifest.Teardown.TimeoutSeconds > 0 {
		return time.Duration(c.manifest.Teardown.TimeoutSeconds) * time.Second
	}
	return 30 * time.Second
}

func (c *Coordinator) emit(event TeardownEvent) {
	if c.eventListener != nil {
		c.eventListener(event)
	}
}

// ExecuteHook runs a single lifecycle hook command within its configured timeout.
func ExecuteHook(ctx context.Context, environment env.HostEnvironment, hook manifest.HookConfig) error {
	cmdStr := strings.TrimSpace(hook.Command)
	name := strings.TrimSpace(hook.Name)
	if name == "" {
		name = cmdStr
	}

	if cmdStr == "" {
		if hook.IgnoreErrors {
			return nil
		}
		return fmt.Errorf("hook %q has empty command", name)
	}

	timeoutSec := hook.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	timeout := time.Duration(timeoutSec) * time.Second

	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := environment.Exec(hookCtx, "sh", "-c", cmdStr)
	if err != nil {
		if hook.IgnoreErrors {
			return nil
		}
		if errors.Is(hookCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("hook %q timed out after %v: %w", name, timeout, err)
		}
		return fmt.Errorf("hook %q failed: %w (output: %s)", name, err, strings.TrimSpace(string(out)))
	}

	return nil
}

// ExecuteHooks runs a sequence of lifecycle hooks in order.
func ExecuteHooks(ctx context.Context, environment env.HostEnvironment, hooks []manifest.HookConfig) error {
	for _, hook := range hooks {
		if err := ExecuteHook(ctx, environment, hook); err != nil {
			return err
		}
	}
	return nil
}

// ParseDirtyKB extracts the Dirty field value in kilobytes from /proc/meminfo content.
func ParseDirtyKB(data []byte) (int64, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "dirty:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("malformed Dirty entry in meminfo: %q", line)
			}
			val, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid numeric value for Dirty in meminfo: %w", err)
			}
			if val < 0 {
				return 0, fmt.Errorf("negative Dirty value in meminfo: %d", val)
			}
			return val, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading meminfo data: %w", err)
	}
	return 0, errors.New("Dirty field not found in meminfo")
}
