package cli

import (
	"context"
	"errors"
	"fmt"

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

	m, resolvedPath, code := r.loadAndValidateManifest(args, jsonOutput, true)
	if code != ExitOK {
		return code
	}

	if !dryRun && r.Env.Geteuid() != 0 {
		return r.emitPrivilegeError(jsonOutput)
	}

	installer := systemd.NewInstaller(r.Env)
	data, err := installer.Install(ctx, m, systemd.InstallOptions{
		DryRun:     dryRun,
		ConfigPath: resolvedPath,
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
	unit1 := systemd.DefaultServiceName
	unit2 := systemd.DefaultTimerName
	if len(data.Units) >= 2 {
		unit1 = data.Units[0]
		unit2 = data.Units[1]
	}
	fmt.Fprintf(r.Stdout, "  ✔ [2/4] Compiled units: %s, %s\n", unit1, unit2)
	fmt.Fprintf(r.Stdout, "  ✔ [3/4] Verified syntax: systemd-analyze verify (0 errors)\n")

	if dryRun {
		if data.UnitContents != nil {
			if svc, ok := data.UnitContents[systemd.DefaultServiceName]; ok {
				fmt.Fprintf(r.Stdout, "\n--- %s ---\n%s", systemd.DefaultServiceName, svc)
			}
			if tmr, ok := data.UnitContents[systemd.DefaultTimerName]; ok {
				fmt.Fprintf(r.Stdout, "\n--- %s ---\n%s", systemd.DefaultTimerName, tmr)
			}
		}
		fmt.Fprintf(r.Stdout, "\n  DRY RUN COMPLETE: Units validated cleanly. No changes written to disk.\n")
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "  ✔ [4/4] Activated units: systemctl daemon-reload && systemctl enable --now %s\n", systemd.DefaultTimerName)
	fmt.Fprintf(r.Stdout, "\n  ✔ Installation successful! Next poweroff: %s\n", data.NextTrigger)
	return ExitOK
}
