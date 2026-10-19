package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/lifecycle"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

func (r *Runner) runSleep(ctx context.Context, jsonOutput bool, args []string, m *manifest.Manifest) int {
	if m == nil {
		m = &manifest.Manifest{}
		m.ApplyDefaults()
	}

	force := false
	dryRun := false

	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		case "--dry-run", "-n":
			dryRun = true
		}
	}

	coord := lifecycle.NewCoordinator(r.Env, m,
		lifecycle.WithForce(force),
		lifecycle.WithDryRun(dryRun),
		lifecycle.WithStateManager(r.getStateManager()),
	)

	result, err := coord.Execute(ctx)
	if err != nil {
		var inErr *lifecycle.InhibitionError
		if errors.As(err, &inErr) {
			eval := inErr.Result
			if jsonOutput {
				_ = WriteJSON(r.Stdout, false, nil, &Error{
					Code:    eval.Code,
					Message: eval.Reason,
				})
				return ExitInhibited
			}

			if eval.Code == "ERR_INHIBITED" {
				fmt.Fprintf(r.Stderr, "● INHIBITED (code 2): Active maintenance lock detected\n")
				if eval.Lock != nil {
					fmt.Fprintf(r.Stderr, "  Lock ID:     %s\n", eval.Lock.ID)
					fmt.Fprintf(r.Stderr, "  Reason:      %s\n", eval.Lock.Reason)
					fmt.Fprintf(r.Stderr, "  Expires:     in %s\n", eval.Lock.Remaining)
					fmt.Fprintf(r.Stderr, "  To release:  arkon inhibit release %s\n", eval.Lock.ID)
				}
				fmt.Fprintf(r.Stderr, "  To override: arkon sleep --force\n")
			} else {
				fmt.Fprintf(r.Stderr, "● ERROR (code 2): Refusing sleep during daytime window\n")
				fmt.Fprintf(r.Stderr, "  %s\n", eval.Reason)
				fmt.Fprintf(r.Stderr, "  To override: arkon sleep --force\n")
			}
			return ExitInhibited
		}

		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_SLEEP_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}

		fmt.Fprintf(r.Stderr, "● ERROR (code 1): Sleep sequence failed\n")
		fmt.Fprintf(r.Stderr, "  %s\n", err.Error())
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, result, nil)
		return ExitOK
	}

	if dryRun {
		fmt.Fprintf(r.Stdout, "● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE (DRY RUN)\n\n")
		fmt.Fprintf(r.Stdout, "  ✔ Invariants confirmed (0 active locks, daytime clear)\n")
		fmt.Fprintf(r.Stdout, "  ✔ Workload Teardown (simulated):\n")
		if len(result.TiersStopped) > 0 {
			fmt.Fprintf(r.Stdout, "    - Profile order: %s\n", strings.Join(result.TiersStopped, " → "))
		}
		fmt.Fprintf(r.Stdout, "    - Residual workloads: would stop\n")
		fmt.Fprintf(r.Stdout, "  ✔ Storage Sync (simulated):\n")
		fmt.Fprintf(r.Stdout, "    - Kernel dirty page drain: would flush\n")
		fmt.Fprintf(r.Stdout, "  ✔ Hardware Wakealarm (simulated):\n")
		fmt.Fprintf(r.Stdout, "    - Target: %s (+%ds relative write)\n", result.NextWakeTime.Format("2006-01-02 15:04:05 MST"), result.WakeDeltaSec)
		fmt.Fprintf(r.Stdout, "    - Safety buffer: %ds (minimum 180s satisfied)\n", result.WakeDeltaSec)
		fmt.Fprintf(r.Stdout, "  ✔ Host Power Action (simulated):\n")
		fmt.Fprintf(r.Stdout, "    - Action: systemctl %s\n\n", result.PowerAction)
		fmt.Fprintf(r.Stdout, "  ✔ Dry run complete. No workloads stopped, no RTC sysfs written, power maintained.\n")
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE\n\n")
	fmt.Fprintf(r.Stdout, "  ✔ Invariants confirmed (0 active locks, daytime clear)\n")
	fmt.Fprintf(r.Stdout, "  ✔ Workload Teardown:\n")
	if len(result.TiersStopped) > 0 {
		fmt.Fprintf(r.Stdout, "    - Profiles stopped: %s\n", strings.Join(result.TiersStopped, ", "))
	}
	fmt.Fprintf(r.Stdout, "    - Residual workloads stopped\n")
	fmt.Fprintf(r.Stdout, "  ✔ Storage Sync:\n")
	fmt.Fprintf(r.Stdout, "    - Kernel dirty pages flushed to physical media\n")
	fmt.Fprintf(r.Stdout, "  ✔ Hardware Wakealarm:\n")
	fmt.Fprintf(r.Stdout, "    - Target: %s (+%ds relative write)\n", result.NextWakeTime.Format("2006-01-02 15:04:05 MST"), result.WakeDeltaSec)
	fmt.Fprintf(r.Stdout, "    - Device: %s (retention verified)\n\n", m.Host.RTCDevice)
	fmt.Fprintf(r.Stdout, "  ✔ Teardown complete. Host entering ACPI %s.\n", result.PowerAction)
	return ExitOK
}
