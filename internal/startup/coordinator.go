package startup

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/teardown"
)

// EventType identifies transitions in startup orchestration.
type EventType string

const (
	EventPostWakeHooksStarted   EventType = "post_wake_hooks_started"
	EventPostWakeHookStarted    EventType = "post_wake_hook_started"
	EventPostWakeHookCompleted  EventType = "post_wake_hook_completed"
	EventPostWakeHooksCompleted EventType = "post_wake_hooks_completed"
	EventStartupStarted         EventType = "startup_started"
	EventStartupCompleted       EventType = "startup_completed"
	EventDryRunCompleted        EventType = "dry_run_completed"
)

// StartupEvent records progress events.
type StartupEvent struct {
	Type    EventType
	Hook    string
	Stack   string
	Profile string
	Err     error
}

// StartupResult represents the outcome of an up operation.
type StartupResult struct {
	Status     string   `json:"status"`
	DryRun     bool     `json:"dry_run,omitempty"`
	HooksRun   []string `json:"hooks_run"`
	StacksUp   []string `json:"stacks_up"`
	ProfilesUp []string `json:"profiles_up"`
}

// Coordinator orchestrates boot post-wake hooks and container startups.
type Coordinator struct {
	env           env.HostEnvironment
	manifest      *manifest.Manifest
	provider      StartupProvider
	dryRun        bool
	logger        io.Writer
	verbose       bool
	eventListener func(StartupEvent)
}

// CoordinatorOption customizes coordinator behavior.
type CoordinatorOption func(*Coordinator)

// WithDryRun toggles simulation mode without running hooks or starting containers.
func WithDryRun(dryRun bool) CoordinatorOption {
	return func(c *Coordinator) {
		c.dryRun = dryRun
	}
}

// WithVerbose toggles verbose diagnostic traces.
func WithVerbose(verbose bool) CoordinatorOption {
	return func(c *Coordinator) {
		c.verbose = verbose
	}
}

// WithLogger sets the writer destination for diagnostic logs.
func WithLogger(w io.Writer) CoordinatorOption {
	return func(c *Coordinator) {
		c.logger = w
	}
}

// WithProvider injects a custom startup provider for testing.
func WithProvider(p StartupProvider) CoordinatorOption {
	return func(c *Coordinator) {
		c.provider = p
	}
}

// WithEventListener attaches a listener for startup events.
func WithEventListener(l func(StartupEvent)) CoordinatorOption {
	return func(c *Coordinator) {
		c.eventListener = l
	}
}

// NewCoordinator constructs a startup coordinator.
func NewCoordinator(environment env.HostEnvironment, m *manifest.Manifest, opts ...CoordinatorOption) *Coordinator {
	if m == nil {
		m = &manifest.Manifest{}
	}
	m.ApplyDefaults()

	c := &Coordinator{
		env:      environment,
		manifest: m,
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.provider == nil {
		var stackPaths []string
		for _, s := range m.Startup.ComposeStacks {
			if s.Path != "" {
				stackPaths = append(stackPaths, s.Path)
			}
		}
		c.provider = NewDockerComposeStartupProvider(environment, stackPaths...)
	}

	return c
}

func (c *Coordinator) emit(ev StartupEvent) {
	if c.eventListener != nil {
		c.eventListener(ev)
	}
}

// Execute runs the startup pipeline: post-wake hooks first, then container startup.
func (c *Coordinator) Execute(ctx context.Context) (*StartupResult, error) {
	result := &StartupResult{
		Status: "completed",
		DryRun: c.dryRun,
	}

	for _, h := range c.manifest.Hooks.PostWake {
		result.HooksRun = append(result.HooksRun, h.Name)
	}
	for _, s := range c.manifest.Startup.ComposeStacks {
		if s.Path != "" {
			result.StacksUp = append(result.StacksUp, s.Path)
		}
	}
	result.ProfilesUp = c.manifest.Startup.Profiles

	if c.verbose && c.logger != nil {
		fmt.Fprintf(c.logger, "[startup] initiating boot coordinator (dry_run: %v, stacks: %d, profiles: %d)\n",
			c.dryRun, len(result.StacksUp), len(result.ProfilesUp))
	}

	// 1. Dry run early exit
	if c.dryRun {
		c.emit(StartupEvent{Type: EventDryRunCompleted})
		return result, nil
	}

	// 2. Post-wake hooks
	if len(c.manifest.Hooks.PostWake) > 0 {
		c.emit(StartupEvent{Type: EventPostWakeHooksStarted})
		for _, hook := range c.manifest.Hooks.PostWake {
			if c.verbose && c.logger != nil {
				fmt.Fprintf(c.logger, "[startup] executing post-wake hook %q\n", hook.Name)
			}
			c.emit(StartupEvent{Type: EventPostWakeHookStarted, Hook: hook.Name})
			if err := teardown.ExecuteHook(ctx, c.env, hook); err != nil {
				c.emit(StartupEvent{Type: EventPostWakeHookCompleted, Hook: hook.Name, Err: err})
				return nil, fmt.Errorf("post-wake hook %q failed: %w", hook.Name, err)
			}
			c.emit(StartupEvent{Type: EventPostWakeHookCompleted, Hook: hook.Name})
		}
		c.emit(StartupEvent{Type: EventPostWakeHooksCompleted})
	}

	// 3. Workload startup
	if !c.manifest.Startup.IsEnabled() {
		if c.verbose && c.logger != nil {
			fmt.Fprintf(c.logger, "[startup] startup orchestration disabled in manifest\n")
		}
		return result, nil
	}

	timeout := time.Duration(c.manifest.Startup.TimeoutSeconds) * time.Second
	c.emit(StartupEvent{Type: EventStartupStarted})

	targets := result.StacksUp
	if len(targets) == 0 {
		targets = []string{""}
	}

	for _, stack := range targets {
		if c.verbose && c.logger != nil {
			fmt.Fprintf(c.logger, "[startup] starting stack %q (profiles: %s)\n", stack, strings.Join(result.ProfilesUp, ", "))
		}
		if err := c.provider.StartStack(ctx, stack, result.ProfilesUp, timeout); err != nil {
			return nil, fmt.Errorf("workload startup failed: %w", err)
		}
	}

	c.emit(StartupEvent{Type: EventStartupCompleted})
	return result, nil
}
