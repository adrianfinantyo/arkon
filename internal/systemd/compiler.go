package systemd

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/adrianfinantyo/arkon/internal/manifest"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// DefaultBinaryPath resolves the binary path to use for ExecStart.
func DefaultBinaryPath() string {
	if exe, err := os.Executable(); err == nil && !strings.Contains(exe, "/tmp/go-build") && !strings.HasSuffix(exe, ".test") {
		return exe
	}
	return "/usr/local/bin/arkon"
}

// FormatOnCalendar formats circadian days and sleep time into a valid systemd OnCalendar expression.
func FormatOnCalendar(days []string, sleepAt string) string {
	timeStr := strings.TrimSpace(sleepAt)
	if strings.Count(timeStr, ":") == 1 {
		timeStr += ":00"
	}

	dayStr := strings.Join(days, ",")
	if len(days) == 7 || len(days) == 0 {
		dayStr = "Mon..Sun"
	}
	return fmt.Sprintf("%s *-*-* %s", dayStr, timeStr)
}

// Compile generates native systemd service and timer unit files from a Manifest.
func Compile(m *manifest.Manifest, opts CompileOptions) (*CompiledUnits, error) {
	if m == nil {
		return nil, ErrInvalidManifest
	}

	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		binaryPath = DefaultBinaryPath()
	}
	if !filepath.IsAbs(binaryPath) {
		if abs, err := filepath.Abs(binaryPath); err == nil {
			binaryPath = abs
		}
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "/etc/arkon/arkon.yaml"
	}
	if !filepath.IsAbs(configPath) {
		if abs, err := filepath.Abs(configPath); err == nil {
			configPath = abs
		}
	}

	onCalendar := FormatOnCalendar(m.Circadian.Days, m.Circadian.SleepAt)

	tmplData := struct {
		BinaryPath string
		ConfigPath string
		OnCalendar string
	}{
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		OnCalendar: onCalendar,
	}

	serviceTmpl, err := template.ParseFS(templateFS, "templates/arkon-sleep.service.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse service template: %w", err)
	}

	var serviceBuf bytes.Buffer
	if err := serviceTmpl.Execute(&serviceBuf, tmplData); err != nil {
		return nil, fmt.Errorf("failed to execute service template: %w", err)
	}

	timerTmpl, err := template.ParseFS(templateFS, "templates/arkon-sleep.timer.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse timer template: %w", err)
	}

	var timerBuf bytes.Buffer
	if err := timerTmpl.Execute(&timerBuf, tmplData); err != nil {
		return nil, fmt.Errorf("failed to execute timer template: %w", err)
	}

	bootEnabled := m.Startup.IsEnabled()
	var bootContent string
	if bootEnabled {
		bootTmpl, err := template.ParseFS(templateFS, "templates/arkon-boot.service.tmpl")
		if err != nil {
			return nil, fmt.Errorf("failed to parse boot service template: %w", err)
		}
		var bootBuf bytes.Buffer
		if err := bootTmpl.Execute(&bootBuf, tmplData); err != nil {
			return nil, fmt.Errorf("failed to execute boot service template: %w", err)
		}
		bootContent = bootBuf.String()
	}

	return &CompiledUnits{
		ServiceName:        DefaultServiceName,
		TimerName:          DefaultTimerName,
		BootServiceName:    DefaultBootServiceName,
		ServicePath:        filepath.Join(DefaultTargetDir, DefaultServiceName),
		TimerPath:          filepath.Join(DefaultTargetDir, DefaultTimerName),
		BootServicePath:    filepath.Join(DefaultTargetDir, DefaultBootServiceName),
		ServiceContent:     serviceBuf.String(),
		TimerContent:       timerBuf.String(),
		BootServiceContent: bootContent,
		BootEnabled:        bootEnabled,
	}, nil
}
