package teardown

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

// ProviderNameDockerCompose is the canonical identifier for the Docker Compose teardown provider.
const ProviderNameDockerCompose = "docker-compose"

// TeardownProvider abstracts stopping workloads across distinct runtime environments.
type TeardownProvider interface {
	Name() string
	CheckLiveness(ctx context.Context) error
	StopTier(ctx context.Context, tier string, timeout time.Duration) error
	StopResidual(ctx context.Context, timeout time.Duration) error
}

// DockerComposeProvider manages phased stopping of Docker Compose stacks.
type DockerComposeProvider struct {
	env    env.HostEnvironment
	stacks []string
}

// NewDockerComposeProvider constructs a provider targeting Docker Compose stacks.
func NewDockerComposeProvider(environment env.HostEnvironment, stacks ...string) *DockerComposeProvider {
	return &DockerComposeProvider{
		env:    environment,
		stacks: stacks,
	}
}

// NewDockerComposeProviderFromManifest constructs a provider from manifest TeardownConfig.
func NewDockerComposeProviderFromManifest(environment env.HostEnvironment, cfg manifest.TeardownConfig) *DockerComposeProvider {
	paths := make([]string, len(cfg.ComposeStacks))
	for i, stack := range cfg.ComposeStacks {
		paths[i] = stack.Path
	}
	return NewDockerComposeProvider(environment, paths...)
}

// Name returns the provider identifier.
func (p *DockerComposeProvider) Name() string {
	return ProviderNameDockerCompose
}

// CheckLiveness verifies that the docker compose CLI is accessible.
func (p *DockerComposeProvider) CheckLiveness(ctx context.Context) error {
	out, err := p.env.Exec(ctx, "docker", "compose", "version")
	if err != nil {
		return fmt.Errorf("docker compose is not available: %w (output: %s)", err, string(out))
	}
	return nil
}

// StopTier stops containers matching the specified profile tier across configured stacks.
func (p *DockerComposeProvider) StopTier(ctx context.Context, tier string, timeout time.Duration) error {
	trimmedTier := strings.TrimSpace(tier)
	if trimmedTier == "" {
		return errors.New("tier name cannot be empty")
	}

	targets := p.stacks
	if len(targets) == 0 {
		targets = []string{""}
	}

	for _, stack := range targets {
		args := []string{"compose"}
		if stack != "" {
			args = append(args, "--project-directory", stack)
		}
		args = append(args, "--profile", trimmedTier, "stop")
		if timeout > 0 {
			sec := int(timeout.Seconds())
			if sec <= 0 {
				sec = 1
			}
			args = append(args, "-t", strconv.Itoa(sec))
		}

		out, err := p.env.Exec(ctx, "docker", args...)
		if err != nil {
			if stack != "" {
				return fmt.Errorf("failed to stop tier %q for stack %s: %w (output: %s)", trimmedTier, stack, err, strings.TrimSpace(string(out)))
			}
			return fmt.Errorf("failed to stop tier %q: %w (output: %s)", trimmedTier, err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}

// StopResidual stops unprofiled or untagged containers across registered stacks.
func (p *DockerComposeProvider) StopResidual(ctx context.Context, timeout time.Duration) error {
	targets := p.stacks
	if len(targets) == 0 {
		targets = []string{""}
	}

	for _, stack := range targets {
		args := []string{"compose"}
		if stack != "" {
			args = append(args, "--project-directory", stack)
		}
		args = append(args, "--profile", "*", "stop")
		if timeout > 0 {
			sec := int(timeout.Seconds())
			if sec <= 0 {
				sec = 1
			}
			args = append(args, "-t", strconv.Itoa(sec))
		}

		out, err := p.env.Exec(ctx, "docker", args...)
		if err != nil {
			if stack != "" {
				return fmt.Errorf("failed to stop residual containers for stack %s: %w (output: %s)", stack, err, strings.TrimSpace(string(out)))
			}
			return fmt.Errorf("failed to stop residual containers: %w (output: %s)", err, strings.TrimSpace(string(out)))
		}
	}

	return nil
}
