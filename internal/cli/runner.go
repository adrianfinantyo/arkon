package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/state"
)

// VersionInfo contains build-time metadata.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Runner coordinates CLI argument parsing, environment abstraction, and command execution.
type Runner struct {
	Env      env.HostEnvironment
	Stdout   io.Writer
	Stderr   io.Writer
	Version  VersionInfo
	StateDir string
	Verbose  bool
}

// NewRunner creates a new CLI runner.
func NewRunner(env env.HostEnvironment, stdout, stderr io.Writer, version VersionInfo) *Runner {
	return &Runner{
		Env:     env,
		Stdout:  stdout,
		Stderr:  stderr,
		Version: version,
	}
}

// ParseConfigFlag returns the explicit manifest path passed via -c, --config, or --config=, or empty string.
func (r *Runner) ParseConfigFlag(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-c" || arg == "--config" {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	return ""
}

// ParseConfigPath scans command arguments for -c or --config flags, defaulting to "./arkon.yaml".
func (r *Runner) ParseConfigPath(args []string) string {
	if flag := r.ParseConfigFlag(args); flag != "" {
		return flag
	}
	return "./arkon.yaml"
}

// LoadManifest resolves, parses, and validates the manifest for the runner's environment.
func (r *Runner) LoadManifest(args []string) (*manifest.Manifest, string, error) {
	explicitPath := r.ParseConfigFlag(args)
	return manifest.Load(r.Env, explicitPath)
}

// Run executes a command based on parsed arguments and returns a standard exit code.
func (r *Runner) Run(ctx context.Context, args []string) int {
	jsonOutput, verbose, helpRequested, subcommand, subArgs := r.parseArgs(args)
	r.Verbose = verbose

	if helpRequested || subcommand == "help" {
		r.printUsage(r.Stdout)
		return ExitOK
	}

	if subcommand == "" {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_NO_COMMAND",
				Message: "No command specified. Run 'arkon --help' for usage.",
			})
			return ExitGeneralError
		}
		r.printUsage(r.Stderr)
		return ExitGeneralError
	}

	switch subcommand {
	case "version":
		return r.runVersion(jsonOutput)
	case "schema":
		return r.runSchema(jsonOutput)
	case "apply":
		return r.runApply(ctx, jsonOutput, args, subArgs)
	case "status":
		m, _, code := r.loadAndValidateManifest(args, jsonOutput, false)
		if code != ExitOK {
			return code
		}
		return r.runStatus(ctx, m, jsonOutput, subArgs)
	case "inhibit":
		m, _, code := r.loadAndValidateManifest(args, jsonOutput, false)
		if code != ExitOK {
			return code
		}
		return r.runInhibit(jsonOutput, subArgs, m)
	case "sleep":
		m, _, code := r.loadAndValidateManifest(args, jsonOutput, false)
		if code != ExitOK {
			return code
		}
		return r.runSleep(ctx, jsonOutput, subArgs, m)
	case "wake":
		m, _, code := r.loadAndValidateManifest(args, jsonOutput, false)
		if code != ExitOK {
			return code
		}
		return r.runWake(ctx, jsonOutput, subArgs, m)
	case "up":
		m, _, code := r.loadAndValidateManifest(args, jsonOutput, false)
		if code != ExitOK {
			return code
		}
		return r.runUp(ctx, jsonOutput, subArgs, m)
	default:
		return r.runUnknown(subcommand, jsonOutput)
	}
}

func (r *Runner) loadAndValidateManifest(args []string, jsonOutput bool, required bool) (*manifest.Manifest, string, int) {
	explicitConfig := r.ParseConfigFlag(args)
	if explicitConfig != "" {
		m, path, err := manifest.Load(r.Env, explicitConfig)
		if err != nil {
			return nil, "", r.emitManifestError(err, jsonOutput)
		}
		return m, path, ExitOK
	}

	candidatePath, err := manifest.DiscoverPath(r.Env, "")
	if err == nil {
		m, path, err := manifest.Load(r.Env, candidatePath)
		if err != nil {
			return nil, "", r.emitManifestError(err, jsonOutput)
		}
		return m, path, ExitOK
	}

	if required {
		return nil, "", r.emitManifestError(err, jsonOutput)
	}

	return nil, "", ExitOK
}

func (r *Runner) emitManifestError(err error, jsonOutput bool) int {
	if valErr, ok := err.(*manifest.ValidationError); ok {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_MANIFEST_VALIDATION",
				Message: valErr.Error(),
				Details: valErr,
			})
			return ExitValidationFailure
		}
		fmt.Fprintf(r.Stderr, "Error: %s\n", valErr.Error())
		return ExitValidationFailure
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, false, nil, &Error{
			Code:    "ERR_MANIFEST_LOAD",
			Message: err.Error(),
		})
		return ExitValidationFailure
	}

	fmt.Fprintf(r.Stderr, "Error: %s\n", err.Error())
	return ExitValidationFailure
}

func (r *Runner) parseArgs(args []string) (jsonOutput bool, verbose bool, helpRequested bool, subcommand string, subArgs []string) {
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "-v" || arg == "--verbose" || arg == "--debug":
			verbose = true
		case arg == "-h" || arg == "--help":
			helpRequested = true
		case arg == "-c" || arg == "--config":
			i++
		case strings.HasPrefix(arg, "--config="):
			continue
		default:
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) > 0 {
		subcommand = remaining[0]
		subArgs = remaining[1:]
	}

	return jsonOutput, verbose, helpRequested, subcommand, subArgs
}

func (r *Runner) resolveStateDir() string {
	if r.StateDir != "" {
		return r.StateDir
	}
	if r.Env != nil {
		if v, ok := r.Env.LookupEnv("ARKON_STATE_DIR"); ok && v != "" {
			return v
		}
	}
	return state.DefaultStateDir
}

func (r *Runner) getStateManager() *state.Manager {
	dir := r.resolveStateDir()
	var opts []state.Option
	if r.Env != nil {
		opts = append(opts, state.WithClock(r.Env.Now))
	}
	if r.Stderr != nil {
		opts = append(opts, state.WithWarningWriter(r.Stderr))
	}
	return state.NewManager(dir, opts...)
}

func (r *Runner) invokingUser() string {
	if r.Env != nil {
		if u, ok := r.Env.LookupEnv("SUDO_USER"); ok && u != "" {
			return u
		}
		if u, ok := r.Env.LookupEnv("USER"); ok && u != "" {
			return u
		}
	}
	return "operator"
}

func (r *Runner) runVersion(jsonOutput bool) int {
	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, r.Version, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "arkon %s (commit: %s, built: %s)\n",
		r.Version.Version, r.Version.Commit, r.Version.Date)
	return ExitOK
}

func (r *Runner) runSchema(jsonOutput bool) int {
	schema := CanonicalSchema()

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, schema, nil)
		return ExitOK
	}

	enc := json.NewEncoder(r.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(schema)
	return ExitOK
}

func (r *Runner) runUnknown(cmd string, jsonOutput bool) int {
	msg := fmt.Sprintf("Unknown command: %s. Run 'arkon --help' for available commands.", cmd)

	if jsonOutput {
		_ = WriteJSON(r.Stdout, false, nil, &Error{
			Code:    "ERR_UNKNOWN_COMMAND",
			Message: msg,
		})
		return ExitGeneralError
	}

	fmt.Fprintf(r.Stderr, "Error: %s\n\n", msg)
	r.printUsage(r.Stderr)
	return ExitGeneralError
}

func (r *Runner) printUsage(w io.Writer) {
	fmt.Fprintf(w, `Arkon: Lightweight Host Lifecycle & Docker Stack Controller

Usage:
  arkon [command] [flags]

Available Commands:
  apply      Compile arkon.yaml and sync host systemd units
  status     Display host power state, RTC alarm, and timer status
  sleep      Execute graceful profile teardown and program RTC wakealarm
  up         Execute post-wake hooks and start declared container workloads
  wake       Execute post-wake lifecycle hooks
  inhibit    Temporarily pause automated scheduled shutdown
  schema     Emit canonical JSON Schema (Draft 2020-12) for manifests and state
  version    Print Arkon version information
  help       Show command help

Flags:
  -c, --config string   Path to arkon.yaml manifest (default: "./arkon.yaml")
  --dry-run             Simulate execution without modifying system state
  --force               Override daytime protection and active inhibition locks
  --json                Output typed JSON envelope
  -v, --verbose         Enable verbose diagnostic logging (also --debug)
  -h, --help            Show help for command

Exit Codes:
  0: Success / OK
  1: General Error
  2: Power Transition Inhibited (Active Lock or Daytime Protection)
  3: Manifest Validation or Compilation Failure
  4: Insufficient Privileges (Root required without --dry-run)
`)
}
