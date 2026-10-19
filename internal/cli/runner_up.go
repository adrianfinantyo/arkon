package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/startup"
)

func (r *Runner) runUp(ctx context.Context, jsonOutput bool, args []string, m *manifest.Manifest) int {
	if m == nil {
		m = &manifest.Manifest{}
		m.ApplyDefaults()
	}

	dryRun := false
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--verbose", "-v", "--debug":
			r.Verbose = true
		}
	}

	opts := []startup.CoordinatorOption{
		startup.WithDryRun(dryRun),
	}
	if r.Verbose {
		opts = append(opts, startup.WithVerbose(true), startup.WithLogger(r.Stderr))
	}

	coord := startup.NewCoordinator(r.Env, m, opts...)
	result, err := coord.Execute(ctx)
	if err != nil {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_STARTUP_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "● ERROR (code 1): Startup sequence failed\n")
		fmt.Fprintf(r.Stderr, "  %s\n", err.Error())
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, result, nil)
		return ExitOK
	}

	if dryRun {
		fmt.Fprintf(r.Stdout, "● ARKON BOOT & WORKLOAD STARTUP (DRY RUN)\n\n")
		if len(result.HooksRun) > 0 {
			fmt.Fprintf(r.Stdout, "  ✔ Post-wake hooks (simulated): %s\n", strings.Join(result.HooksRun, ", "))
		}
		fmt.Fprintf(r.Stdout, "  ✔ Workload startup (simulated):\n")
		if len(result.StacksUp) > 0 {
			fmt.Fprintf(r.Stdout, "    - Stacks: %s\n", strings.Join(result.StacksUp, ", "))
		}
		if len(result.ProfilesUp) > 0 {
			fmt.Fprintf(r.Stdout, "    - Profiles: %s\n", strings.Join(result.ProfilesUp, " → "))
		}
		fmt.Fprintf(r.Stdout, "\n  ✔ Dry run complete. No hooks executed, no containers started.\n")
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● ARKON BOOT & WORKLOAD STARTUP\n\n")
	if len(result.HooksRun) > 0 {
		fmt.Fprintf(r.Stdout, "  ✔ Executed post-wake hooks (%d hooks): %s\n", len(result.HooksRun), strings.Join(result.HooksRun, ", "))
	}
	if len(result.ProfilesUp) > 0 {
		fmt.Fprintf(r.Stdout, "  ✔ Started container profiles: %s\n", strings.Join(result.ProfilesUp, " → "))
	} else if len(result.StacksUp) > 0 {
		fmt.Fprintf(r.Stdout, "  ✔ Started container stacks (%d stacks)\n", len(result.StacksUp))
	} else {
		fmt.Fprintf(r.Stdout, "  ✔ Startup complete (no stacks configured)\n")
	}
	return ExitOK
}
