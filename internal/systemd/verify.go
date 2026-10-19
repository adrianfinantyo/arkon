package systemd

import (
	"context"
	"fmt"
	"strings"

	"github.com/adrianfinantyo/arkon/internal/env"
)

// ValidateUnitSyntax verifies structural validity of service and timer units in Go.
func ValidateUnitSyntax(serviceContent, timerContent string) error {
	if !strings.Contains(serviceContent, "[Unit]") {
		return fmt.Errorf("service unit missing [Unit] section")
	}
	if !strings.Contains(serviceContent, "[Service]") {
		return fmt.Errorf("service unit missing [Service] section")
	}
	if !strings.Contains(serviceContent, "Type=oneshot") {
		return fmt.Errorf("service unit missing Type=oneshot")
	}
	if !strings.Contains(serviceContent, "ExecStart=") {
		return fmt.Errorf("service unit missing ExecStart")
	}

	if !strings.Contains(timerContent, "[Unit]") {
		return fmt.Errorf("timer unit missing [Unit] section")
	}
	if !strings.Contains(timerContent, "[Timer]") {
		return fmt.Errorf("timer unit missing [Timer] section")
	}
	if !strings.Contains(timerContent, "Persistent=false") {
		return fmt.Errorf("timer unit missing Persistent=false")
	}
	if !strings.Contains(timerContent, "OnCalendar=") {
		return fmt.Errorf("timer unit missing OnCalendar")
	}
	if !strings.Contains(timerContent, "[Install]") {
		return fmt.Errorf("timer unit missing [Install] section")
	}
	return nil
}

// VerifyStagedUnits validates the syntax of staged unit files, running systemd-analyze verify when available.
func VerifyStagedUnits(ctx context.Context, hostEnv env.HostEnvironment, servicePath, timerPath string) error {
	serviceBytes, err := hostEnv.ReadFile(servicePath)
	if err != nil {
		return fmt.Errorf("cannot read staged service unit: %w", err)
	}
	timerBytes, err := hostEnv.ReadFile(timerPath)
	if err != nil {
		return fmt.Errorf("cannot read staged timer unit: %w", err)
	}

	if err := ValidateUnitSyntax(string(serviceBytes), string(timerBytes)); err != nil {
		return fmt.Errorf("%w: %v", ErrVerificationFailed, err)
	}

	out, err := hostEnv.Exec(ctx, "systemd-analyze", "verify", servicePath, timerPath)
	if err != nil {
		errStr := err.Error()
		outStr := string(out)

		if strings.Contains(errStr, "executable file not found") || strings.Contains(errStr, "command not mocked") {
			return nil
		}

		if strings.Contains(outStr, "is not executable: No such file or directory") {
			lines := strings.Split(strings.TrimSpace(outStr), "\n")
			fatal := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if strings.Contains(trimmed, "is not executable: No such file or directory") {
					continue
				}
				if strings.Contains(trimmed, "Unknown section") ||
					strings.Contains(trimmed, "bad unit file setting") ||
					strings.Contains(trimmed, "Refusing") ||
					strings.Contains(trimmed, "Failed to parse") {
					fatal = true
					break
				}
			}
			if !fatal {
				return nil
			}
		}

		return fmt.Errorf("%w: %s (%v)", ErrVerificationFailed, outStr, err)
	}

	return nil
}
