package cli

import (
	"context"
	"fmt"

	"github.com/adrianfinantyo/arkon/internal/lifecycle"
	"github.com/adrianfinantyo/arkon/internal/manifest"
)

func (r *Runner) runWake(ctx context.Context, jsonOutput bool, args []string, m *manifest.Manifest) int {
	coord := lifecycle.NewCoordinator(r.Env, m,
		lifecycle.WithStateManager(r.getStateManager()),
	)

	if err := coord.ExecutePostWake(ctx); err != nil {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_POST_WAKE_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: post-wake hooks failed: %v\n", err)
		return ExitGeneralError
	}

	hookCount := 0
	if m != nil {
		hookCount = len(m.Hooks.PostWake)
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, map[string]any{
			"status":         "completed",
			"hooks_executed": hookCount,
		}, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● Post-wake lifecycle sequence completed (%d hooks executed)\n", hookCount)
	return ExitOK
}
