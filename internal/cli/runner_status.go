package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/status"
)

func (r *Runner) runStatus(ctx context.Context, m *manifest.Manifest, jsonOutput bool, args []string) int {
	stateDir := r.resolveStateDir()

	data, err := status.Collect(ctx, r.Env, m, stateDir)
	if err != nil {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_STATUS_COLLECTION",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: %s\n", err.Error())
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, data, nil)
		return ExitOK
	}

	noColor := false
	if r.Env != nil {
		if _, ok := r.Env.LookupEnv("NO_COLOR"); ok {
			noColor = true
		}
	}
	if !noColor {
		if f, ok := r.Stdout.(*os.File); ok {
			if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
				noColor = true
			}
		}
	}

	output := status.FormatStatus(data, noColor)
	fmt.Fprint(r.Stdout, output)
	return ExitOK
}
