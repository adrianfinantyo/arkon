package startup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
)

// StartupProvider abstracts workload bring-up mechanisms.
type StartupProvider interface {
	Name() string
	CheckLiveness(ctx context.Context) error
	StartStack(ctx context.Context, stack string, profiles []string, timeout time.Duration) error
}

// DockerComposeStartupProvider manages starting Docker Compose services.
type DockerComposeStartupProvider struct {
	env    env.HostEnvironment
	stacks []string
}

// NewDockerComposeStartupProvider constructs a startup provider.
func NewDockerComposeStartupProvider(environment env.HostEnvironment, stacks ...string) *DockerComposeStartupProvider {
	return &DockerComposeStartupProvider{
		env:    environment,
		stacks: stacks,
	}
}

// Name returns the provider identifier.
func (p *DockerComposeStartupProvider) Name() string {
	return "docker-compose"
}

// CheckLiveness verifies Docker Compose is callable on the host.
func (p *DockerComposeStartupProvider) CheckLiveness(ctx context.Context) error {
	out, err := p.env.Exec(ctx, "docker", "compose", "version")
	if err != nil {
		return fmt.Errorf("docker compose is not available: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// StartStack brings up services in a stack with declared profiles in order, or all if profiles are empty.
func (p *DockerComposeStartupProvider) StartStack(ctx context.Context, stack string, profiles []string, timeout time.Duration) error {
	if len(profiles) > 0 {
		for _, profile := range profiles {
			trimmed := strings.TrimSpace(profile)
			if trimmed == "" {
				continue
			}
			args := []string{"compose"}
			if stack != "" {
				args = append(args, "--project-directory", stack)
			}
			args = append(args, "--profile", trimmed, "up", "-d")
			if timeout > 0 {
				sec := int(timeout.Seconds())
				if sec <= 0 {
					sec = 1
				}
				args = append(args, "-t", strconv.Itoa(sec))
			}
			out, err := p.env.Exec(ctx, "docker", args...)
			if err != nil {
				return fmt.Errorf("failed to start profile %q for stack %s: %w (output: %s)", trimmed, stack, err, strings.TrimSpace(string(out)))
			}
		}
		return nil
	}

	args := []string{"compose"}
	if stack != "" {
		args = append(args, "--project-directory", stack)
	}
	args = append(args, "up", "-d")
	if timeout > 0 {
		sec := int(timeout.Seconds())
		if sec <= 0 {
			sec = 1
		}
		args = append(args, "-t", strconv.Itoa(sec))
	}
	out, err := p.env.Exec(ctx, "docker", args...)
	if err != nil {
		return fmt.Errorf("failed to start stack %s: %w (output: %s)", stack, err, strings.TrimSpace(string(out)))
	}
	return nil
}
