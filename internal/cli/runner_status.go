package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/status"
)

func (r *Runner) runStatus(ctx context.Context, jsonOutput bool, args []string) int {
	stateDir := r.resolveStateDir()
	explicitConfig := r.ParseConfigFlag(args)

	res, err := manifest.ResolveManifest(r.Env, explicitConfig, stateDir)

	var m *manifest.Manifest
	var opts []status.CollectOption

	if res != nil {
		if res.BrokenActive {
			opts = append(opts, status.WithActiveManifest(res.ActivePath))
			opts = append(opts, status.WithManifestError("active manifest file is missing or unreadable"))
		} else if res.Path != "" {
			if dataBytes, err := r.Env.ReadFile(res.Path); err == nil {
				if parsed, err := manifest.ParseAndValidate(dataBytes); err == nil {
					m = parsed
					opts = append(opts, status.WithActiveManifest(res.Path))
				} else {
					if explicitConfig != "" {
						return r.emitManifestError(err, jsonOutput)
					}
					opts = append(opts, status.WithActiveManifest(res.Path))
					opts = append(opts, status.WithManifestError(err.Error()))
				}
			} else {
				if explicitConfig != "" {
					return r.emitManifestError(err, jsonOutput)
				}
				opts = append(opts, status.WithActiveManifest(res.Path))
				opts = append(opts, status.WithManifestError(err.Error()))
			}
		}
		if res.DriftNotice != "" {
			opts = append(opts, status.WithDriftNotice(res.DriftNotice))
		}
	} else if explicitConfig != "" && err != nil {
		return r.emitManifestError(err, jsonOutput)
	}

	data, err := status.Collect(ctx, r.Env, m, stateDir, opts...)
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
