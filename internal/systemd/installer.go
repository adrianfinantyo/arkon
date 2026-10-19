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
	stagedBoot := filepath.Join(stagingDir, compiled.BootServiceName)
	bootDest := filepath.Join(targetDir, compiled.BootServiceName)

	defer func() {
		_ = i.Env.RemoveFile(stagedService)
		_ = i.Env.RemoveFile(stagedTimer)
		if compiled.BootEnabled {
			_ = i.Env.RemoveFile(stagedBoot)
		}
		_ = os.Remove(stagingDir)
	}()

	if err := i.Env.WriteFile(stagedService, []byte(compiled.ServiceContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write staging service unit: %w", err)
	}
	if err := i.Env.WriteFile(stagedTimer, []byte(compiled.TimerContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write staging timer unit: %w", err)
	}

	var extraStaged []string
	if compiled.BootEnabled {
		if err := i.Env.WriteFile(stagedBoot, []byte(compiled.BootServiceContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write staging boot unit: %w", err)
		}
		extraStaged = append(extraStaged, stagedBoot)
	}

	if err := VerifyStagedUnits(ctx, i.Env, stagedService, stagedTimer, extraStaged...); err != nil {
		return nil, err
	}

	nextTrigger := formatNextTrigger(m.Circadian.SleepAt, m.Host.Timezone)

	unitsList := []string{serviceDest, timerDest}
	unitContentsMap := map[string]string{
		compiled.ServiceName: compiled.ServiceContent,
		compiled.TimerName:   compiled.TimerContent,
	}
	if compiled.BootEnabled {
		unitsList = append(unitsList, bootDest)
		unitContentsMap[compiled.BootServiceName] = compiled.BootServiceContent
	}

	if opts.DryRun {
		return &ApplyData{
			Manifest:      opts.ConfigPath,
			DryRun:        true,
			Validated:     true,
			Units:         unitsList,
			UnitContents:  unitContentsMap,
			SystemdReload: false,
			TimerEnabled:  false,
			BootEnabled:   compiled.BootEnabled,
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

	var bootBackup []byte
	hadBoot := false
	if compiled.BootEnabled {
		if data, err := i.Env.ReadFile(bootDest); err == nil {
			hadBoot = true
			bootBackup = data
		}
	}

	serviceTmp := serviceDest + ".tmp"
	timerTmp := timerDest + ".tmp"
	bootTmp := bootDest + ".tmp"
	defer func() {
		_ = i.Env.RemoveFile(serviceTmp)
		_ = i.Env.RemoveFile(timerTmp)
		if compiled.BootEnabled {
			_ = i.Env.RemoveFile(bootTmp)
		}
	}()

	rollback := func() {
		_ = i.Env.RemoveFile(serviceTmp)
		_ = i.Env.RemoveFile(timerTmp)
		if compiled.BootEnabled {
			_ = i.Env.RemoveFile(bootTmp)
		}
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
		if compiled.BootEnabled {
			if hadBoot {
				_ = i.Env.WriteFile(bootDest, bootBackup, 0644)
			} else {
				_ = i.Env.RemoveFile(bootDest)
			}
		}
		_ = i.Env.SyncFS()
		_, _ = i.Env.Exec(ctx, "systemctl", "daemon-reload")
	}

	if err := i.Env.WriteFile(serviceTmp, []byte(compiled.ServiceContent), 0644); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to write temporary service unit: %w", err)
	}
	if err := i.Env.WriteFile(timerTmp, []byte(compiled.TimerContent), 0644); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to write temporary timer unit: %w", err)
	}
	if compiled.BootEnabled {
		if err := i.Env.WriteFile(bootTmp, []byte(compiled.BootServiceContent), 0644); err != nil {
			rollback()
			return nil, fmt.Errorf("failed to write temporary boot unit: %w", err)
		}
	}

	if err := i.Env.RenameFile(serviceTmp, serviceDest); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to atomically commit service unit: %w", err)
	}
	if err := i.Env.RenameFile(timerTmp, timerDest); err != nil {
		rollback()
		return nil, fmt.Errorf("failed to atomically commit timer unit: %w", err)
	}
	if compiled.BootEnabled {
		if err := i.Env.RenameFile(bootTmp, bootDest); err != nil {
			rollback()
			return nil, fmt.Errorf("failed to atomically commit boot unit: %w", err)
		}
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

	if compiled.BootEnabled {
		if out, err := i.Env.Exec(ctx, "systemctl", "enable", compiled.BootServiceName); err != nil {
			rollback()
			return nil, fmt.Errorf("systemctl enable %s failed: %s (%w)", compiled.BootServiceName, string(out), err)
		}
	}

	return &ApplyData{
		Manifest:      opts.ConfigPath,
		DryRun:        false,
		Validated:     true,
		Units:         unitsList,
		UnitContents:  unitContentsMap,
		SystemdReload: true,
		TimerEnabled:  true,
		BootEnabled:   compiled.BootEnabled,
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
