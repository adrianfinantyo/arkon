package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"time"

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
	jsonOutput, helpRequested, subcommand, subArgs := r.parseArgs(args)

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
	case "apply", "status":
		if _, _, code := r.loadAndValidateManifest(args, jsonOutput); code != ExitOK {
			return code
		}
		return r.runNotImplemented(subcommand, jsonOutput, subArgs)
	case "inhibit":
		if _, _, code := r.loadAndValidateManifest(args, jsonOutput); code != ExitOK {
			return code
		}
		return r.runInhibit(jsonOutput, subArgs)
	case "sleep":
		if _, _, code := r.loadAndValidateManifest(args, jsonOutput); code != ExitOK {
			return code
		}
		return r.runSleep(jsonOutput, subArgs)
	default:
		return r.runUnknown(subcommand, jsonOutput)
	}
}

func (r *Runner) loadAndValidateManifest(args []string, jsonOutput bool) (*manifest.Manifest, string, int) {
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

func (r *Runner) parseArgs(args []string) (jsonOutput bool, helpRequested bool, subcommand string, subArgs []string) {
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "-h" || arg == "--help":
			helpRequested = true
		case arg == "-c" || arg == "--config":
			// Skip flag and its value for subcommand identification
			i++
		case strings.HasPrefix(arg, "--config="):
			// Skip inline config flag
			continue
		default:
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) > 0 {
		subcommand = remaining[0]
		subArgs = remaining[1:]
	}

	return jsonOutput, helpRequested, subcommand, subArgs
}

func (r *Runner) getStateManager() *state.Manager {
	dir := r.StateDir
	if dir == "" {
		if r.Env != nil {
			if v, ok := r.Env.LookupEnv("ARKON_STATE_DIR"); ok && v != "" {
				dir = v
			}
		}
		if dir == "" {
			dir = os.Getenv("ARKON_STATE_DIR")
		}
		if dir == "" {
			dir = state.DefaultStateDir
		}
	}

	var opts []state.Option
	if r.Env != nil {
		opts = append(opts, state.WithClock(r.Env.Now))
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
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "operator"
}

func (r *Runner) runInhibit(jsonOutput bool, args []string) int {
	// Check for list subcommand or flag
	if len(args) > 0 && args[0] == "list" {
		return r.runInhibitList(jsonOutput)
	}
	for _, arg := range args {
		if arg == "--list" {
			return r.runInhibitList(jsonOutput)
		}
	}

	// Check for release subcommand or flag
	if len(args) > 0 && args[0] == "release" {
		id := ""
		if len(args) > 1 {
			id = args[1]
		}
		return r.runInhibitRelease(jsonOutput, id)
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--release" {
			id := ""
			if i+1 < len(args) {
				id = args[i+1]
			}
			return r.runInhibitRelease(jsonOutput, id)
		}
		if strings.HasPrefix(arg, "--release=") {
			return r.runInhibitRelease(jsonOutput, strings.TrimPrefix(arg, "--release="))
		}
	}

	// Parse flags for creating a lock
	reason := "Manual maintenance window"
	durationStr := "2h"

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-r" || arg == "--reason":
			if i+1 < len(args) {
				reason = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--reason="):
			reason = strings.TrimPrefix(arg, "--reason=")
		case arg == "-d" || arg == "--duration":
			if i+1 < len(args) {
				durationStr = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--duration="):
			durationStr = strings.TrimPrefix(arg, "--duration=")
		}
	}

	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		msg := fmt.Sprintf("Invalid duration %q: %v", durationStr, err)
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_INVALID_DURATION",
				Message: msg,
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: %s\n", msg)
		return ExitGeneralError
	}

	mgr := r.getStateManager()
	lock, err := mgr.AcquireLock(reason, dur, r.invokingUser())
	if err != nil {
		msg := fmt.Sprintf("Failed to acquire lock: %v", err)
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_ACQUIRE_LOCK_FAILED",
				Message: msg,
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: %s\n", msg)
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, lock, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● Inhibition Lock Created: %s\n", lock.ID)
	fmt.Fprintf(r.Stdout, "  Reason:    %s\n", lock.Reason)
	fmt.Fprintf(r.Stdout, "  Duration:  %s (expires at %s)\n", state.FormatRemaining(dur), lock.ExpiresAt.Format("15:04:05 MST"))
	fmt.Fprintf(r.Stdout, "  To cancel: arkon inhibit release %s\n", lock.ID)
	return ExitOK
}

func (r *Runner) runInhibitList(jsonOutput bool) int {
	mgr := r.getStateManager()
	locks, err := mgr.ListLocks()
	if err != nil {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_LIST_LOCKS_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: failed to list locks: %v\n", err)
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, locks, nil)
		return ExitOK
	}

	if len(locks) == 0 {
		fmt.Fprintf(r.Stdout, "● Maintenance Locks: None active (operational)\n")
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "● Maintenance Locks:\n")
	for _, l := range locks {
		fmt.Fprintf(r.Stdout, "  - %s: %q (expires in %s)\n", l.ID, l.Reason, l.Remaining)
	}
	return ExitOK
}

func (r *Runner) runInhibitRelease(jsonOutput bool, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		msg := "Lock ID is required for release. Run 'arkon inhibit list' to view active locks."
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_MISSING_ARGUMENT",
				Message: msg,
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: %s\n", msg)
		return ExitGeneralError
	}

	mgr := r.getStateManager()
	err := mgr.ReleaseLock(id)
	if err != nil {
		if errors.Is(err, state.ErrLockNotFound) {
			msg := fmt.Sprintf("inhibition lock '%s' not found", id)
			if jsonOutput {
				_ = WriteJSON(r.Stdout, false, nil, &Error{
					Code:    "ERR_LOCK_NOT_FOUND",
					Message: msg,
				})
				return ExitGeneralError
			}
			fmt.Fprintf(r.Stderr, "Error: %s\n", msg)
			return ExitGeneralError
		}

		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_RELEASE_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error: failed to release lock: %v\n", err)
		return ExitGeneralError
	}

	if jsonOutput {
		_ = WriteJSON(r.Stdout, true, map[string]string{
			"id":     id,
			"status": "released",
		}, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "✔ Maintenance lock '%s' released. Circadian schedule resumed.\n", id)
	return ExitOK
}

func (r *Runner) runSleep(jsonOutput bool, args []string) int {
	force := false
	simulateInhibit := false
	simulateDaytime := false

	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		case "--simulate-inhibit":
			simulateInhibit = true
		case "--simulate-daytime":
			simulateDaytime = true
		}
	}

	mgr := r.getStateManager()
	activeLocks, err := mgr.ListLocks()
	if err != nil && !simulateInhibit {
		if jsonOutput {
			_ = WriteJSON(r.Stdout, false, nil, &Error{
				Code:    "ERR_STATE_READ_FAILED",
				Message: err.Error(),
			})
			return ExitGeneralError
		}
		fmt.Fprintf(r.Stderr, "Error reading state: %v\n", err)
		return ExitGeneralError
	}

	if simulateInhibit && len(activeLocks) == 0 {
		activeLocks = []state.InhibitionLock{
			{
				ID:        "inh-simulated",
				Reason:    "Simulated maintenance override",
				CreatedBy: "simulation",
				ExpiresAt: r.Env.Now().Add(1 * time.Hour),
				Remaining: "1h",
			},
		}
	}

	daytimeCfg := state.DaytimeConfig{
		WakeAt:            "07:00",
		SleepAt:           "23:00",
		DaytimeProtection: simulateDaytime,
	}

	checkTime := r.Env.Now()
	if simulateDaytime {
		checkTime = time.Date(checkTime.Year(), checkTime.Month(), checkTime.Day(), 14, 0, 0, 0, checkTime.Location())
	}

	eval := state.EvaluateSleepAllowed(checkTime, daytimeCfg, activeLocks, force)
	if eval.Inhibited {
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
		_ = WriteJSON(r.Stdout, true, map[string]any{
			"status": "completed",
			"forced": force,
		}, nil)
		return ExitOK
	}

	fmt.Fprintf(r.Stdout, "✔ Sleep sequence initiated.\n")
	return ExitOK
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

func (r *Runner) runNotImplemented(cmd string, jsonOutput bool, subArgs []string) int {
	msg := fmt.Sprintf("Command '%s' is not implemented yet.", cmd)

	if jsonOutput {
		_ = WriteJSON(r.Stdout, false, nil, &Error{
			Code:    "ERR_NOT_IMPLEMENTED",
			Message: msg,
		})
		return ExitGeneralError
	}

	fmt.Fprintf(r.Stderr, "Error: %s\n", msg)
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
  inhibit    Temporarily pause automated scheduled shutdown
  schema     Emit canonical JSON Schema (Draft 2020-12) for manifests and state
  version    Print Arkon version information
  help       Show command help

Flags:
  -c, --config string   Path to arkon.yaml manifest (default: "./arkon.yaml")
  --json                Output typed JSON envelope
  -h, --help            Show help for command

Exit Codes:
  0: Success / OK
  1: General Error
  2: Power Transition Inhibited (Active Lock or Daytime Protection)
  3: Manifest Validation or Compilation Failure
  4: Insufficient Privileges (Root required without --dry-run)
`)
}
