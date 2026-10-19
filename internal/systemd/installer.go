package systemd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

// Installer manages the atomic installation, verification, and rollback of systemd units.
type Installer struct {
	Env env.HostEnvironment
}

// NewInstaller creates an Installer using the provided host environment.
func NewInstaller(hostEnv env.HostEnvironment) *Installer {
	return &Installer{Env: hostEnv}
}

// Install compiles, stages, verifies, and atomically commits systemd units for a manifest.
func (i *Installer) Install(ctx context.Context, m *manifest.Manifest, opts InstallOptions) (*ApplyData, error) {
	if !opts.DryRun && i.Env.Geteuid() != 0 {
		return nil, ErrInsufficientPrivileges
	}

	compiled, err := Compile(m, CompileOptions{
		BinaryPath: opts.BinaryPath,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	targetDir := opts.TargetDir
	if targetDir == "" {
		targetDir = DefaultTargetDir
	}
	serviceDest := filepath.Join(targetDir, compiled.ServiceName)
	timerDest := filepath.Join(targetDir, compiled.TimerName)

	stagingDir := opts.StagingDir
	if stagingDir == "" {
		stagingDir = filepath.Join(os.TempDir(), fmt.Sprintf("arkon-stage-%d", i.Env.Now().UnixNano()))
	}
	stagedService := filepath.Join(stagingDir, compiled.ServiceName)
	stagedTimer := filepath.Join(stagingDir, compiled.TimerName)

	defer func() {
		_ = i.Env.RemoveFile(stagedService)
		_ = i.Env.RemoveFile(stagedTimer)
		_ = os.Remove(stagingDir)
	}()

	if err := i.Env.WriteFile(stagedService, []byte(compiled.ServiceContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write staging service unit: %w", err)
	}
	if err := i.Env.WriteFile(stagedTimer, []byte(compiled.TimerContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write staging timer unit: %w", err)
	}

	if err := VerifyStagedUnits(ctx, i.Env, stagedService, stagedTimer); err != nil {
		return nil, err
	}

	nextTrigger := formatNextTrigger(m.Circadian.SleepAt, m.Host.Timezone)

	if opts.DryRun {
		return &ApplyData{
			Manifest:      opts.ConfigPath,
			DryRun:        true,
			Validated:     true,
			Units:         []string{serviceDest, timerDest},
			UnitContents:  map[string]string{
				compiled.ServiceName: compiled.ServiceContent,
				compiled.TimerName:   compiled.TimerContent,
			},
			SystemdReload: false,
			TimerEnabled:  false,
			NextTrigger:   nextTrigger,
		}, nil
	}

	var serviceBackup []byte
	hadService := false
	if data, err := i.Env.ReadFile(serviceDest); err == nil {
		hadService = true
		serviceBackup = data
	}

	var timerBackup []byte
	hadTimer := false
	if data, err := i.Env.ReadFile(timerDest); err == nil {
		hadTimer = true
		timerBackup = data
	}

	rollback := func() {
		if hadService {
			_ = i.Env.WriteFile(serviceDest, serviceBackup, 0644)
		} else {
			_ = i.Env.RemoveFile(serviceDest)
		}
		if hadTimer {
			_ = i.Env.WriteFile(timerDest, timerBackup, 0644)
		} else {
			_ = i.Env.RemoveFile(timerDest)
		}
		_ = i.Env.SyncFS()
		_, _ = i.Env.Exec(ctx, "systemctl", "daemon-reload")
	}

	if err := i.Env.WriteFile(serviceDest, []byte(compiled.ServiceContent), 0644); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to install service unit: %w", err)
	}
	if err := i.Env.WriteFile(timerDest, []byte(compiled.TimerContent), 0644); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to install timer unit: %w", err)
	}

	if err := i.Env.SyncFS(); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to sync filesystem: %w", err)
	}

	if out, err := i.Env.Exec(ctx, "systemctl", "daemon-reload"); err != nil {
		rollback()
		return nil, fmt.Errorf("systemctl daemon-reload failed: %s (%w)", string(out), err)
	}

	if out, err := i.Env.Exec(ctx, "systemctl", "enable", "--now", compiled.TimerName); err != nil {
		rollback()
		return nil, fmt.Errorf("systemctl enable --now %s failed: %s (%w)", compiled.TimerName, string(out), err)
	}

	return &ApplyData{
		Manifest:      opts.ConfigPath,
		DryRun:        false,
		Validated:     true,
		Units:         []string{serviceDest, timerDest},
		UnitContents:  map[string]string{
			compiled.ServiceName: compiled.ServiceContent,
			compiled.TimerName:   compiled.TimerContent,
		},
		SystemdReload: true,
		TimerEnabled:  true,
		NextTrigger:   nextTrigger,
	}, nil
}

func formatNextTrigger(sleepAt, timezone string) string {
	timeStr := strings.TrimSpace(sleepAt)
	if strings.Count(timeStr, ":") == 1 {
		timeStr += ":00"
	}
	if timezone != "" {
		return fmt.Sprintf("%s %s", timeStr, timezone)
	}
	return timeStr
}
