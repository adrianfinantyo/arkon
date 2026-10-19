package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adrianfinantyo/arkon/internal/manifest"
	"github.com/adrianfinantyo/arkon/internal/state"
)

func (r *Runner) runInhibit(jsonOutput bool, args []string, m *manifest.Manifest) int {
	if len(args) > 0 && args[0] == "list" {
		return r.runInhibitList(jsonOutput)
	}
	for _, arg := range args {
		if arg == "--list" {
			return r.runInhibitList(jsonOutput)
		}
	}

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

	reason := "Manual maintenance window"
	durationStr := "2h"
	if m != nil && m.Inhibition.DefaultDuration != "" {
		durationStr = m.Inhibition.DefaultDuration
	}

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
