package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/env"
)

// VersionInfo contains build-time metadata.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Runner coordinates CLI argument parsing, environment abstraction, and command execution.
type Runner struct {
	Env     env.HostEnvironment
	Stdout  io.Writer
	Stderr  io.Writer
	Version VersionInfo
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

// ParseConfigPath scans command arguments for -c or --config flags, defaulting to "./arkon.yaml".
func (r *Runner) ParseConfigPath(args []string) string {
	defaultPath := "./arkon.yaml"
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
	return defaultPath
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
	case "apply", "status", "sleep", "inhibit":
		return r.runNotImplemented(subcommand, jsonOutput, subArgs)
	default:
		return r.runUnknown(subcommand, jsonOutput)
	}
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
