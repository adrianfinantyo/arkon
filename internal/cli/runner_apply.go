package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/systemd"
)

func (r *Runner) emitPrivilegeError(jsonOutput bool) int {
	if jsonOutput {
		_ = WriteJSON(r.Stdout, false, nil, &Error{
			Code:    "ERR_INSUFFICIENT_PERMISSIONS",
			Message: "Applying systemd units requires root privileges (run with sudo or pass --dry-run)",
		})
		return ExitInsufficientPrivilege
	}
	fmt.Fprintf(r.Stderr, "● ERROR (code 4): Root privileges required to write /etc/systemd/system/\n")
	fmt.Fprintf(r.Stderr, "  Arkon requires elevated access to install systemd service & timer units.\n")
	fmt.Fprintf(r.Stderr, "  • Apply with sudo:      sudo arkon apply\n")
	fmt.Fprintf(r.Stderr, "  • Inspect without root: arkon apply --dry-run\n")
	return ExitInsufficientPrivilege
}

func (r *Runner) runApply(ctx context.Context, jsonOutput bool, args []string, subArgs []string) int {
	dryRun := false
	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
			break
		}
	}

	explicitConfig := r.ParseConfigFlag(args)
	stateDir := r.resolveStateDir()
	resolvedPath, err := manifest.ResolveApplySource(r.Env, explicitConfig, stateDir)
	if err != nil {
		return r.emitManifestError(err, jsonOutput)
	}

	dataBytes, err := r.Env.ReadFile(resolvedPath)
	if err != nil {
		return r.emitManifestError(fmt.Errorf("failed to read manifest at %s: %w", resolvedPath, err), jsonOutput)
	}

	m, err := manifest.ParseAndValidate(dataBytes)
	if err != nil {
		return r.emitManifestError(err, jsonOutput)
	}

	if !dryRun && r.Env.Geteuid() != 0 {
		return r.emitPrivilegeError(jsonOutput)
	}

	installer := systemd.NewInstaller(r.Env)
	data, err := installer.Install(ctx, m, systemd.InstallOptions{
		DryRun:     dryRun,
		ConfigPath: resolvedPath,
		StateDir:   stateDir,
		AppliedBy:  r.invokingUser(),
	})
	if err != nil {
		if errors.Is(err, systemd.ErrInsufficientPrivileges) {
			return r.emitPrivilegeError(jsonOutput)
		}
		if errors.Is(err, systemd.ErrVerificationFailed) || errors.Is(err, systemd.ErrInvalidManifest) {
			if jsonOutput {
				_ = WriteJSON(r.Stdout, false, nil, &Error{
					Code:    "ERR_COMPILATION_FAILED",
					Message: err.Error(),
				})
				return ExitValidationFailure
			}
			fmt.Fprintf(r.Stderr, "Error: compilation or verification failed: %v\n", err)
			return ExitValidationFailure
		}
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_APPLY_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: %v\n", err)
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, data, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● ARKON SYSTEMD COMPILER\n")
	fmt.Fprintf(r.Stdout, "  ✔ [1/4] Manifest parsed: %s (version: %s, provider: %s)\n", resolvedPath, m.Version, m.Teardown.Provider)
	fmt.Fprintf(r.Stdout, "  ✔ [2/4] Compiled units: %s\n", strings.Join(data.Units, ", "))
	fmt.Fprintf(r.Stdout, "  ✔ [3/4] Verified syntax: systemd-analyze verify (0 errors)\n")

	if dryRun {
		if data.UnitContents != nil {
			if svc, ok := data.UnitContents[systemd.DefaultServiceName]; ok {
				fmt.Fprintf(r.Stdout, "\n--- %s ---\n%s", systemd.DefaultServiceName, svc)
			}
			if tmr, ok := data.UnitContents[systemd.DefaultTimerName]; ok {
				fmt.Fprintf(r.Stdout, "\n--- %s ---\n%s", systemd.DefaultTimerName, tmr)
			}
			if boot, ok := data.UnitContents[systemd.DefaultBootServiceName]; ok {
				fmt.Fprintf(r.Stdout, "\n--- %s ---\n%s", systemd.DefaultBootServiceName, boot)
			}
		}
		fmt.Fprintf(r.Stdout, "\n  DRY RUN COMPLETE: Units validated cleanly. No changes written to disk.\n")
		return ExitOK
	}

	step := 4
	totalSteps := 4
	if data.ActiveManifest != "" && data.ActiveManifest != data.CanonicalLink {
		totalSteps = 5
		fmt.Fprintf(r.Stdout, "  ✔ [%d/%d] Registered active manifest: %s -> %s\n", step, totalSteps, data.ActiveManifest, data.CanonicalLink)
		step++
	}

	if data.BootEnabled {
		fmt.Fprintf(r.Stdout, "  ✔ [%d/%d] Activated units: systemctl daemon-reload && systemctl enable --now %s && systemctl enable %s\n", step, totalSteps, systemd.DefaultTimerName, systemd.DefaultBootServiceName)
	} else {
		fmt.Fprintf(r.Stdout, "  ✔ [%d/%d] Activated units: systemctl daemon-reload && systemctl enable --now %s\n", step, totalSteps, systemd.DefaultTimerName)
	}
	fmt.Fprintf(r.Stdout, "\n  ✔ Installation successful! Next poweroff: %s\n", data.NextTrigger)
	return ExitOK
}
