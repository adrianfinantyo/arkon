package teardown_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/teardown"
)

func TestDockerComposeProvider_NameAndLiveness(t *testing.T) {
	t.Run("returns expected provider name", func(t *testing.T) {
		mem := env.NewMemEnv()
		provider := teardown.NewDockerComposeProvider(mem)
		if got := provider.Name(); got != "docker-compose" {
			t.Fatalf("expected provider name 'docker-compose', got %q", got)
		}
	})

	t.Run("check liveness succeeds when docker compose responds", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name == "docker" && len(args) == 2 && args[0] == "compose" && args[1] == "version" {
				return []byte("Docker Compose version v2.24.5"), nil
			}
			return nil, errors.New("unexpected command")
		}

		provider := teardown.NewDockerComposeProvider(mem)
		if err := provider.CheckLiveness(context.Background()); err != nil {
			t.Fatalf("expected liveness check to succeed, got %v", err)
		}
	})

	t.Run("check liveness fails when docker compose command errors", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("docker daemon is not running")
		}

		provider := teardown.NewDockerComposeProvider(mem)
		if err := provider.CheckLiveness(context.Background()); err == nil {
			t.Fatal("expected liveness check to fail, got nil")
		}
	})

	t.Run("constructs provider with stacks", func(t *testing.T) {
		mem := env.NewMemEnv()
		cfg := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps", "/opt/stacks/infra")
		if cfg.Name() != "docker-compose" {
			t.Fatalf("expected provider name 'docker-compose', got %q", cfg.Name())
		}
	})
}

func TestDockerComposeProvider_StopTier(t *testing.T) {
	t.Run("executes stop command with profile and timeout for single stack", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executedCommands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			fullCmd := append([]string{name}, args...)
			executedCommands = append(executedCommands, fullCmd)
			return []byte("Stopping container... done\n"), nil
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps")
		err := provider.StopTier(context.Background(), "apps", 15*time.Second)
		if err != nil {
			t.Fatalf("unexpected error stopping tier: %v", err)
		}

		if len(executedCommands) != 1 {
			t.Fatalf("expected 1 command executed, got %d", len(executedCommands))
		}

		expected := []string{"docker", "compose", "--project-directory", "/opt/stacks/apps", "--profile", "apps", "stop", "-t", "15"}
		got := executedCommands[0]
		if len(got) != len(expected) {
			t.Fatalf("expected command %v, got %v", expected, got)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("at index %d: expected %q, got %q", i, expected[i], got[i])
			}
		}
	})

	t.Run("executes stop command without project directory when no stacks configured", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executedCommands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executedCommands = append(executedCommands, append([]string{name}, args...))
			return nil, nil
		}

		provider := teardown.NewDockerComposeProvider(mem)
		err := provider.StopTier(context.Background(), "infra", 30*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(executedCommands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(executedCommands))
		}

		expected := []string{"docker", "compose", "--profile", "infra", "stop", "-t", "30"}
		got := executedCommands[0]
		if len(got) != len(expected) {
			t.Fatalf("expected command %v, got %v", expected, got)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("at index %d: expected %q, got %q", i, expected[i], got[i])
			}
		}
	})

	t.Run("iterates over multiple stacks sequentially", func(t *testing.T) {
		mem := env.NewMemEnv()
		var stoppedDirs []string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			for i, arg := range args {
				if arg == "--project-directory" && i+1 < len(args) {
					stoppedDirs = append(stoppedDirs, args[i+1])
				}
			}
			return nil, nil
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/web", "/opt/stacks/db")
		err := provider.StopTier(context.Background(), "storage", 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(stoppedDirs) != 2 || stoppedDirs[0] != "/opt/stacks/web" || stoppedDirs[1] != "/opt/stacks/db" {
			t.Fatalf("expected sequential stop of [/opt/stacks/web, /opt/stacks/db], got %v", stoppedDirs)
		}
	})

	t.Run("stops and returns error when docker compose command fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		callCount := 0
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			callCount++
			return []byte("container database failed to stop"), errors.New("exit status 1")
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps", "/opt/stacks/infra")
		err := provider.StopTier(context.Background(), "apps", 10*time.Second)
		if err == nil {
			t.Fatal("expected error when tier stop fails, got nil")
		}

		if callCount != 1 {
			t.Fatalf("expected execution to abort after first stack failure, executed %d times", callCount)
		}
	})

	t.Run("rejects empty tier name", func(t *testing.T) {
		mem := env.NewMemEnv()
		provider := teardown.NewDockerComposeProvider(mem)
		if err := provider.StopTier(context.Background(), "", 10*time.Second); err == nil {
			t.Fatal("expected error for empty tier, got nil")
		}
	})
}

func TestDockerComposeProvider_StopResidual(t *testing.T) {
	t.Run("executes catch-all stop with wildcard profile flag for single stack", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executedCommands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executedCommands = append(executedCommands, append([]string{name}, args...))
			return []byte("Stopping untagged containers... done\n"), nil
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps")
		err := provider.StopResidual(context.Background(), 20*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(executedCommands) != 1 {
			t.Fatalf("expected 1 command executed, got %d", len(executedCommands))
		}

		expected := []string{"docker", "compose", "--project-directory", "/opt/stacks/apps", "--profile", "*", "stop", "-t", "20"}
		got := executedCommands[0]
		if len(got) != len(expected) {
			t.Fatalf("expected command %v, got %v", expected, got)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("at index %d: expected %q, got %q", i, expected[i], got[i])
			}
		}
	})

	t.Run("executes residual stop without project directory when no stacks configured", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executedCommands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executedCommands = append(executedCommands, append([]string{name}, args...))
			return nil, nil
		}

		provider := teardown.NewDockerComposeProvider(mem)
		err := provider.StopResidual(context.Background(), 15*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(executedCommands) != 1 {
			t.Fatalf("expected 1 command executed, got %d", len(executedCommands))
		}

		expected := []string{"docker", "compose", "--profile", "*", "stop", "-t", "15"}
		got := executedCommands[0]
		if len(got) != len(expected) {
			t.Fatalf("expected command %v, got %v", expected, got)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("at index %d: expected %q, got %q", i, expected[i], got[i])
			}
		}
	})

	t.Run("iterates over multiple stacks sequentially for residual stop", func(t *testing.T) {
		mem := env.NewMemEnv()
		var stoppedDirs []string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			for i, arg := range args {
				if arg == "--project-directory" && i+1 < len(args) {
					stoppedDirs = append(stoppedDirs, args[i+1])
				}
			}
			return nil, nil
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps", "/opt/stacks/infra")
		err := provider.StopResidual(context.Background(), 10*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(stoppedDirs) != 2 || stoppedDirs[0] != "/opt/stacks/apps" || stoppedDirs[1] != "/opt/stacks/infra" {
			t.Fatalf("expected sequential stop of [/opt/stacks/apps, /opt/stacks/infra], got %v", stoppedDirs)
		}
	})

	t.Run("aborts and returns error when residual stop fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		callCount := 0
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			callCount++
			return []byte("container failed to terminate"), errors.New("exit status 1")
		}

		provider := teardown.NewDockerComposeProvider(mem, "/opt/stacks/apps", "/opt/stacks/infra")
		err := provider.StopResidual(context.Background(), 10*time.Second)
		if err == nil {
			t.Fatal("expected error when residual stop fails, got nil")
		}

		if callCount != 1 {
			t.Fatalf("expected execution to abort after first stack failure, executed %d times", callCount)
		}
	})
}
