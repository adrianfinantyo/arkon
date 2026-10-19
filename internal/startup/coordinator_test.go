package startup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/startup"
)

type mockStartupProvider struct {
	startCalls []string
	startFunc  func(ctx context.Context, stack string, profiles []string, timeout time.Duration) error
}

func (m *mockStartupProvider) Name() string {
	return "mock-startup"
}

func (m *mockStartupProvider) CheckLiveness(ctx context.Context) error {
	return nil
}

func (m *mockStartupProvider) StartStack(ctx context.Context, stack string, profiles []string, timeout time.Duration) error {
	call := stack + ":" + strings.Join(profiles, ",")
	m.startCalls = append(m.startCalls, call)
	if m.startFunc != nil {
		return m.startFunc(ctx, stack, profiles, timeout)
	}
	return nil
}

func TestDockerComposeStartupProvider(t *testing.T) {
	t.Run("starts profiles sequentially for stack", func(t *testing.T) {
		mem := env.NewMemEnv()
		var commands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			commands = append(commands, append([]string{name}, args...))
			return []byte("ok"), nil
		}

		provider := startup.NewDockerComposeStartupProvider(mem, "/opt/apps")
		err := provider.StartStack(context.Background(), "/opt/apps", []string{"proxy", "infra"}, 60*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(commands) != 2 {
			t.Fatalf("expected 2 commands, got %d", len(commands))
		}

		expectedCmd1 := []string{"docker", "compose", "--project-directory", "/opt/apps", "--profile", "proxy", "up", "-d", "-t", "60"}
		if strings.Join(commands[0], " ") != strings.Join(expectedCmd1, " ") {
			t.Errorf("expected %v, got %v", expectedCmd1, commands[0])
		}

		expectedCmd2 := []string{"docker", "compose", "--project-directory", "/opt/apps", "--profile", "infra", "up", "-d", "-t", "60"}
		if strings.Join(commands[1], " ") != strings.Join(expectedCmd2, " ") {
			t.Errorf("expected %v, got %v", expectedCmd2, commands[1])
		}
	})

	t.Run("starts default stack when no profiles specified", func(t *testing.T) {
		mem := env.NewMemEnv()
		var commands [][]string
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			commands = append(commands, append([]string{name}, args...))
			return []byte("ok"), nil
		}

		provider := startup.NewDockerComposeStartupProvider(mem, "/opt/infra")
		err := provider.StartStack(context.Background(), "/opt/infra", nil, 30*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(commands) != 1 {
			t.Fatalf("expected 1 command, got %d", len(commands))
		}
		expectedCmd := []string{"docker", "compose", "--project-directory", "/opt/infra", "up", "-d", "-t", "30"}
		if strings.Join(commands[0], " ") != strings.Join(expectedCmd, " ") {
			t.Errorf("expected %v, got %v", expectedCmd, commands[0])
		}
	})

	t.Run("returns error when docker compose fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("port already in use"), errors.New("exit status 1")
		}

		provider := startup.NewDockerComposeStartupProvider(mem, "/opt/apps")
		err := provider.StartStack(context.Background(), "/opt/apps", []string{"web"}, 30*time.Second)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "port already in use") {
			t.Errorf("expected output in error, got: %v", err)
		}
	})
}

func TestCoordinator_Execute(t *testing.T) {
	t.Run("executes post-wake hooks and starts stacks", func(t *testing.T) {
		mem := env.NewMemEnv()
		var hookExecuted bool
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			fullCmd := name + " " + strings.Join(args, " ")
			if strings.Contains(fullCmd, "manage-failing-hdd.sh") {
				hookExecuted = true
			}
			return []byte("ok"), nil
		}

		mock := &mockStartupProvider{}
		m := &manifest.Manifest{
			Version: "1",
			Hooks: manifest.HooksConfig{
				PostWake: []manifest.HookConfig{
					{Name: "detach-disk", Command: "/usr/local/bin/manage-failing-hdd.sh disable"},
				},
			},
			Startup: manifest.StartupConfig{
				ComposeStacks: []manifest.ComposeStack{{Path: "/opt/homelab"}},
				Profiles:      []string{"proxy", "infra", "apps"},
			},
		}

		coord := startup.NewCoordinator(mem, m, startup.WithProvider(mock))
		result, err := coord.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !hookExecuted {
			t.Error("expected post-wake hook to execute")
		}
		if len(mock.startCalls) != 1 {
			t.Fatalf("expected 1 start call, got %d", len(mock.startCalls))
		}
		if mock.startCalls[0] != "/opt/homelab:proxy,infra,apps" {
			t.Errorf("unexpected start call: %s", mock.startCalls[0])
		}
		if len(result.HooksRun) != 1 || result.HooksRun[0] != "detach-disk" {
			t.Errorf("unexpected hooks run: %v", result.HooksRun)
		}
	})

	t.Run("dry run executes no hooks and no container commands", func(t *testing.T) {
		mem := env.NewMemEnv()
		var executedAny bool
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			executedAny = true
			return []byte("ok"), nil
		}

		mock := &mockStartupProvider{}
		m := &manifest.Manifest{
			Version: "1",
			Hooks: manifest.HooksConfig{
				PostWake: []manifest.HookConfig{{Name: "test-hook", Command: "true"}},
			},
			Startup: manifest.StartupConfig{
				ComposeStacks: []manifest.ComposeStack{{Path: "/opt/homelab"}},
				Profiles:      []string{"apps"},
			},
		}

		coord := startup.NewCoordinator(mem, m,
			startup.WithProvider(mock),
			startup.WithDryRun(true),
		)
		result, err := coord.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if executedAny {
			t.Error("dry run should not execute any command")
		}
		if len(mock.startCalls) != 0 {
			t.Errorf("dry run should not call provider start, got %d calls", len(mock.startCalls))
		}
		if !result.DryRun {
			t.Error("expected DryRun=true in result")
		}
	})

	t.Run("fails when post-wake hook fails", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("disk detach failed"), errors.New("exit status 1")
		}

		mock := &mockStartupProvider{}
		m := &manifest.Manifest{
			Version: "1",
			Hooks: manifest.HooksConfig{
				PostWake: []manifest.HookConfig{{Name: "failing-hook", Command: "false"}},
			},
			Startup: manifest.StartupConfig{
				ComposeStacks: []manifest.ComposeStack{{Path: "/opt/homelab"}},
			},
		}

		coord := startup.NewCoordinator(mem, m, startup.WithProvider(mock))
		_, err := coord.Execute(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if len(mock.startCalls) != 0 {
			t.Error("should not attempt workload start when hook fails")
		}
	})

	t.Run("verbose logging writes to logger", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		}

		mock := &mockStartupProvider{}
		m := &manifest.Manifest{
			Version: "1",
			Hooks: manifest.HooksConfig{
				PostWake: []manifest.HookConfig{{Name: "hook1", Command: "true"}},
			},
			Startup: manifest.StartupConfig{
				ComposeStacks: []manifest.ComposeStack{{Path: "/opt/homelab"}},
				Profiles:      []string{"proxy"},
			},
		}

		var buf strings.Builder
		coord := startup.NewCoordinator(mem, m,
			startup.WithProvider(mock),
			startup.WithVerbose(true),
			startup.WithLogger(&buf),
		)
		_, err := coord.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		logOut := buf.String()
		if !strings.Contains(logOut, "[startup] initiating boot coordinator") {
			t.Errorf("expected initiating log, got: %s", logOut)
		}
		if !strings.Contains(logOut, "[startup] executing post-wake hook \"hook1\"") {
			t.Errorf("expected hook log, got: %s", logOut)
		}
		if !strings.Contains(logOut, "[startup] starting stack \"/opt/homelab\"") {
			t.Errorf("expected starting stack log, got: %s", logOut)
		}
	})
}
