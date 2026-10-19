package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
	"github.com/adrianfinantyo/arkon/internal/systemd"
)

func newTestRunner(env env.HostEnvironment) (*Runner, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	v := VersionInfo{
		Version: "1.0.0-test",
		Commit:  "abc1234",
		Date:    "2026-10-19",
	}
	runner := NewRunner(env, stdout, stderr, v)
	return runner, stdout, stderr
}

func TestExitCodeConstants(t *testing.T) {
	if ExitOK != 0 {
		t.Fatalf("expected ExitOK == 0, got %d", ExitOK)
	}
	if ExitGeneralError != 1 {
		t.Fatalf("expected ExitGeneralError == 1, got %d", ExitGeneralError)
	}
	if ExitInhibited != 2 {
		t.Fatalf("expected ExitInhibited == 2, got %d", ExitInhibited)
	}
	if ExitValidationFailure != 3 {
		t.Fatalf("expected ExitValidationFailure == 3, got %d", ExitValidationFailure)
	}
	if ExitInsufficientPrivilege != 4 {
		t.Fatalf("expected ExitInsufficientPrivilege == 4, got %d", ExitInsufficientPrivilege)
	}
}

func TestEnvelopeSerialization(t *testing.T) {
	// Success envelope with data
	successEnv := Envelope{
		Success: true,
		Data:    map[string]string{"key": "value"},
		Error:   nil,
	}
	data, err := json.Marshal(successEnv)
	if err != nil {
		t.Fatalf("failed to marshal success envelope: %v", err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"success":true`) {
		t.Fatalf("expected success:true in %s", raw)
	}
	if !strings.Contains(raw, `"data":{"key":"value"}`) {
		t.Fatalf("expected data in %s", raw)
	}
	if !strings.Contains(raw, `"error":null`) {
		t.Fatalf("expected error:null in %s", raw)
	}

	// Error envelope with null data
	errEnv := Envelope{
		Success: false,
		Data:    nil,
		Error: &Error{
			Code:    "ERR_UNKNOWN_COMMAND",
			Message: "unknown command test",
		},
	}
	data, err = json.Marshal(errEnv)
	if err != nil {
		t.Fatalf("failed to marshal error envelope: %v", err)
	}
	raw = string(data)
	if !strings.Contains(raw, `"success":false`) {
		t.Fatalf("expected success:false in %s", raw)
	}
	if !strings.Contains(raw, `"data":null`) {
		t.Fatalf("expected data:null in %s", raw)
	}
	if !strings.Contains(raw, `"code":"ERR_UNKNOWN_COMMAND"`) {
		t.Fatalf("expected code in %s", raw)
	}
}

func TestTopLevelHelpFlags(t *testing.T) {
	testCases := [][]string{
		{"-h"},
		{"--help"},
		{"help"},
	}

	requiredSubcommands := []string{
		"apply",
		"status",
		"sleep",
		"inhibit",
		"schema",
		"version",
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc, " "), func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), tc)
			if code != ExitOK {
				t.Fatalf("expected exit code %d, got %d", ExitOK, code)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got: %s", stderr.String())
			}

			output := stdout.String()
			for _, sub := range requiredSubcommands {
				if !strings.Contains(output, sub) {
					t.Errorf("usage output missing subcommand %q in:\n%s", sub, output)
				}
			}
		})
	}
}

func TestNoArgsRunsUsageAndExits1(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d, got %d", ExitGeneralError, code)
	}
	// Output can be on stdout or stderr, but usage must be shown
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "Usage:") {
		t.Fatalf("expected usage info, got: %s", combined)
	}
}

func TestVersionCommand(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"version"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "1.0.0-test") {
		t.Fatalf("expected version 1.0.0-test, got: %s", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Fatalf("expected commit abc1234, got: %s", out)
	}
	if !strings.Contains(out, "2026-10-19") {
		t.Fatalf("expected date 2026-10-19, got: %s", out)
	}
}

func TestVersionCommandJSON(t *testing.T) {
	testCases := [][]string{
		{"version", "--json"},
		{"--json", "version"},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc, " "), func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), tc)
			if code != ExitOK {
				t.Fatalf("expected exit code %d, got %d", ExitOK, code)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got: %s", stderr.String())
			}

			var envelope Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("failed to decode json envelope: %v (raw: %s)", err, stdout.String())
			}

			if !envelope.Success {
				t.Fatalf("expected success: true")
			}
			if envelope.Error != nil {
				t.Fatalf("expected error: nil, got %v", envelope.Error)
			}

			dataMap, ok := envelope.Data.(map[string]any)
			if !ok {
				t.Fatalf("expected map in data, got %T", envelope.Data)
			}
			if dataMap["version"] != "1.0.0-test" {
				t.Fatalf("expected version 1.0.0-test, got %v", dataMap["version"])
			}
			if dataMap["commit"] != "abc1234" {
				t.Fatalf("expected commit abc1234, got %v", dataMap["commit"])
			}
			if dataMap["date"] != "2026-10-19" {
				t.Fatalf("expected date 2026-10-19, got %v", dataMap["date"])
			}
		})
	}
}

func TestSchemaCommand(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"schema"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var schema map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &schema); err != nil {
		t.Fatalf("failed to parse schema JSON: %v", err)
	}

	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("expected Draft 2020-12 schema, got: %v", schema["$schema"])
	}

	// Verify definitions exist for manifest and state
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("expected $defs map in schema, got %T", schema["$defs"])
	}
	if _, ok := defs["manifest"]; !ok {
		t.Fatal("expected manifest definition in $defs")
	}
	if _, ok := defs["state"]; !ok {
		t.Fatal("expected state definition in $defs")
	}
}

func TestSchemaCommandJSON(t *testing.T) {
	testCases := [][]string{
		{"schema", "--json"},
		{"--json", "schema"},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc, " "), func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), tc)
			if code != ExitOK {
				t.Fatalf("expected exit code %d, got %d", ExitOK, code)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got: %s", stderr.String())
			}

			var envelope Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("failed to parse envelope: %v", err)
			}
			if !envelope.Success {
				t.Fatal("expected success: true")
			}
			if envelope.Error != nil {
				t.Fatalf("expected error: nil, got %v", envelope.Error)
			}

			dataMap, ok := envelope.Data.(map[string]any)
			if !ok {
				t.Fatalf("expected map in data, got %T", envelope.Data)
			}
			if dataMap["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("expected Draft 2020-12 schema in envelope data")
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"not-a-real-cmd"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d, got %d", ExitGeneralError, code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got: %s", stdout.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "not-a-real-cmd") {
		t.Fatalf("expected diagnostic mentioning unknown command, got: %s", errOutput)
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	testCases := [][]string{
		{"--json", "not-a-real-cmd"},
		{"not-a-real-cmd", "--json"},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc, " "), func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), tc)
			if code != ExitGeneralError {
				t.Fatalf("expected exit code %d, got %d", ExitGeneralError, code)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected empty stderr, got: %s", stderr.String())
			}

			var envelope Envelope
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("failed to parse json envelope: %v (raw: %s)", err, stdout.String())
			}

			if envelope.Success {
				t.Fatal("expected success: false")
			}
			if envelope.Data != nil {
				t.Fatalf("expected data: null, got %v", envelope.Data)
			}
			if envelope.Error == nil {
				t.Fatal("expected non-nil error in envelope")
			}
			if envelope.Error.Code != "ERR_UNKNOWN_COMMAND" {
				t.Fatalf("expected ERR_UNKNOWN_COMMAND, got %q", envelope.Error.Code)
			}
		})
	}
}

func TestGlobalConfigFlag(t *testing.T) {
	mem := env.NewMemEnv()
	runner, _, _ := newTestRunner(mem)

	configPath := runner.ParseConfigPath([]string{"-c", "/tmp/custom.yaml", "status"})
	if configPath != "/tmp/custom.yaml" {
		t.Fatalf("expected /tmp/custom.yaml, got %q", configPath)
	}

	configPath = runner.ParseConfigPath([]string{"status", "--config", "/opt/arkon.yaml"})
	if configPath != "/opt/arkon.yaml" {
		t.Fatalf("expected /opt/arkon.yaml, got %q", configPath)
	}

	configPath = runner.ParseConfigPath([]string{"status"})
	if configPath != "./arkon.yaml" {
		t.Fatalf("expected default ./arkon.yaml, got %q", configPath)
	}
}

func TestHostEnvironmentSeamUnprivileged(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000 // Non-root user
	mem.CurrentTime = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"version"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "1.0.0-test") {
		t.Fatalf("expected version output, got: %s", stdout.String())
	}
}

func TestNoArgsJSON(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"--json"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d, got %d", ExitGeneralError, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envelope Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode json envelope: %v", err)
	}
	if envelope.Success {
		t.Fatal("expected success: false")
	}
	if envelope.Error == nil || envelope.Error.Code != "ERR_NO_COMMAND" {
		t.Fatalf("expected ERR_NO_COMMAND, got: %v", envelope.Error)
	}
}

func TestHostEnvironmentStateDirAndInvokingUser(t *testing.T) {
	t.Run("state dir from HostEnvironment", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.EnvVars["ARKON_STATE_DIR"] = "/custom/state/dir"
		runner, _, _ := newTestRunner(mem)
		if got := runner.resolveStateDir(); got != "/custom/state/dir" {
			t.Fatalf("expected /custom/state/dir, got %s", got)
		}
	})

	t.Run("invoking user from SUDO_USER", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.EnvVars["SUDO_USER"] = "alice"
		mem.EnvVars["USER"] = "root"
		runner, _, _ := newTestRunner(mem)
		if got := runner.invokingUser(); got != "alice" {
			t.Fatalf("expected alice, got %s", got)
		}
	})

	t.Run("invoking user falls back to USER then operator", func(t *testing.T) {
		mem := env.NewMemEnv()
		mem.EnvVars["USER"] = "bob"
		runner, _, _ := newTestRunner(mem)
		if got := runner.invokingUser(); got != "bob" {
			t.Fatalf("expected bob, got %s", got)
		}

		delete(mem.EnvVars, "USER")
		if got := runner.invokingUser(); got != "operator" {
			t.Fatalf("expected operator, got %s", got)
		}
	})
}

func TestStatusCommandText(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/etc/hostname"] = []byte("testhost\n")
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	expectedSubstrings := []string{
		"● ARKON HOST STATUS",
		"● Circadian Schedule",
		"● Hardware RTC Wakealarm",
		"● Workload Teardown Pipeline",
		"● Maintenance Locks",
		"  Host:        testhost",
		"  Timezone:",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(out, sub) {
			t.Errorf("expected status output to contain %q, got:\n%s", sub, out)
		}
	}
}

func TestStatusCommandJSON(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/etc/hostname"] = []byte("jsonhost\n")
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status", "--json"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envObj Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envObj); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if !envObj.Success {
		t.Fatal("expected success: true in envelope")
	}

	dataMap, ok := envObj.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data map in envelope, got %T", envObj.Data)
	}
	if dataMap["hostname"] != "jsonhost" {
		t.Errorf("expected hostname jsonhost, got %v", dataMap["hostname"])
	}
	if dataMap["host_power_state"] != "ACPI S0 (Operational Active)" {
		t.Errorf("expected host_power_state ACPI S0, got %v", dataMap["host_power_state"])
	}
}

func TestStatusCommandUnprivileged(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status"})
	if code != ExitOK {
		t.Fatalf("expected unprivileged status to return ExitOK, got %d. stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("expected non-empty stdout")
	}
}

func TestStatusCommandConfigFlag(t *testing.T) {
	mem := env.NewMemEnv()
	configContent := `version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "22:00"
  wake_at: "06:00"
teardown:
  profile_order:
    - default
`
	mem.Files["/tmp/custom.yaml"] = []byte(configContent)
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status", "-c", "/tmp/custom.yaml"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "22:00") {
		t.Errorf("expected output to reflect custom circadian schedule 22:00, got:\n%s", stdout.String())
	}
}

func TestStatusCommandInvalidConfig(t *testing.T) {
	mem := env.NewMemEnv()
	mem.Files["/tmp/invalid.yaml"] = []byte("version: \"999\"\n")
	runner, _, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status", "-c", "/tmp/invalid.yaml"})
	if code != ExitValidationFailure {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitValidationFailure, code, stderr.String())
	}
}

func TestStatusCommandNO_COLOR(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EnvVars["NO_COLOR"] = "1"
	runner, stdout, _ := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"status"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", code, ExitOK)
	}
	if strings.Contains(stdout.String(), "\033[") {
		t.Errorf("expected no ANSI color codes with NO_COLOR=1, found some in:\n%s", stdout.String())
	}
}

func TestGlobalConfigInlineFlag(t *testing.T) {
	mem := env.NewMemEnv()
	runner, _, _ := newTestRunner(mem)

	path := runner.ParseConfigPath([]string{"--config=/etc/custom/arkon.yaml", "status"})
	if path != "/etc/custom/arkon.yaml" {
		t.Fatalf("expected /etc/custom/arkon.yaml, got %q", path)
	}
}

func TestInhibitCreateText(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EnvVars["USER"] = "alice"
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"inhibit", "--reason", "Manual maintenance", "--duration", "3h"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Inhibition Lock Created") {
		t.Errorf("expected 'Inhibition Lock Created' in output: %s", out)
	}
	if !strings.Contains(out, "Manual maintenance") {
		t.Errorf("expected reason in output: %s", out)
	}
	if !strings.Contains(out, "3h") {
		t.Errorf("expected duration in output: %s", out)
	}
}

func TestInhibitCreateJSON(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EnvVars["USER"] = "bob"
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"inhibit", "--reason", "Backup run", "--duration", "1h", "--json"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envelope Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse json envelope: %v", err)
	}
	if !envelope.Success {
		t.Fatal("expected success: true")
	}

	dataMap, ok := envelope.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map in data, got %T", envelope.Data)
	}
	if dataMap["reason"] != "Backup run" {
		t.Errorf("expected reason 'Backup run', got %v", dataMap["reason"])
	}
	if dataMap["created_by"] != "bob" {
		t.Errorf("expected created_by 'bob', got %v", dataMap["created_by"])
	}
	if dataMap["remaining"] != "1h" {
		t.Errorf("expected remaining '1h', got %v", dataMap["remaining"])
	}
	if dataMap["id"] == "" || dataMap["expires_at"] == "" {
		t.Errorf("expected non-empty id and expires_at, got id=%v expires_at=%v", dataMap["id"], dataMap["expires_at"])
	}
}

func TestInhibitListEmptyTextAndJSON(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Text mode
	code := runner.Run(context.Background(), []string{"inhibit", "list"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	if !strings.Contains(stdout.String(), "None active") {
		t.Errorf("expected 'None active' in empty list output, got: %s", stdout.String())
	}

	// JSON mode
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "list", "--json"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	var envEmpty Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envEmpty); err != nil {
		t.Fatalf("failed to decode json envelope: %v", err)
	}
	if !envEmpty.Success {
		t.Fatal("expected success: true")
	}
	locksList, ok := envEmpty.Data.([]any)
	if !ok {
		t.Fatalf("expected []any in data, got %T", envEmpty.Data)
	}
	if len(locksList) != 0 {
		t.Errorf("expected 0 locks, got %d", len(locksList))
	}
}

func TestInhibitListActive(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Create lock
	runner.Run(context.Background(), []string{"inhibit", "--reason", "Storage rebuild", "--duration", "4h"})
	stdout.Reset()
	stderr.Reset()

	// List text
	code := runner.Run(context.Background(), []string{"inhibit", "list"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Storage rebuild") {
		t.Errorf("expected lock reason in list output: %s", out)
	}
	if !strings.Contains(out, "4h") {
		t.Errorf("expected remaining duration in list output: %s", out)
	}

	// List JSON
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "list", "--json"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d, got %d", ExitOK, code)
	}
	var envList Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envList); err != nil {
		t.Fatalf("failed to parse json envelope: %v", err)
	}
	locksList, ok := envList.Data.([]any)
	if !ok || len(locksList) != 1 {
		t.Fatalf("expected 1 lock in list, got %v", envList.Data)
	}
}

func TestInhibitRelease(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Create a lock using JSON to get ID
	code := runner.Run(context.Background(), []string{"inhibit", "--reason", "Quick test", "--json"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d creating lock, got %d", ExitOK, code)
	}
	var created Envelope
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	dataMap := created.Data.(map[string]any)
	lockID := dataMap["id"].(string)

	stdout.Reset()
	stderr.Reset()

	// Release by ID
	code = runner.Run(context.Background(), []string{"inhibit", "release", lockID})
	if code != ExitOK {
		t.Fatalf("expected exit code %d on release, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "released") {
		t.Errorf("expected 'released' in stdout: %s", stdout.String())
	}

	// Listing should now be empty
	stdout.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "list", "--json"})
	var empty Envelope
	_ = json.Unmarshal(stdout.Bytes(), &empty)
	locks := empty.Data.([]any)
	if len(locks) != 0 {
		t.Errorf("expected 0 locks after release, got %d", len(locks))
	}
}

func TestInhibitReleaseErrors(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// 1. Missing ID text
	code := runner.Run(context.Background(), []string{"inhibit", "release"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d for missing id, got %d", ExitGeneralError, code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr: %s", stderr.String())
	}

	// 2. Missing ID JSON
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "release", "--json"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d for missing id, got %d", ExitGeneralError, code)
	}
	var envErr Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envErr); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envErr.Success || envErr.Error == nil || envErr.Error.Code != "ERR_MISSING_ARGUMENT" {
		t.Errorf("expected ERR_MISSING_ARGUMENT, got %v", envErr.Error)
	}

	// 3. Not found text
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "release", "inh-nonexistent"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d for nonexistent lock, got %d", ExitGeneralError, code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found' in stderr: %s", stderr.String())
	}

	// 4. Not found JSON
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"inhibit", "release", "inh-nonexistent", "--json"})
	if code != ExitGeneralError {
		t.Fatalf("expected exit code %d for nonexistent lock, got %d", ExitGeneralError, code)
	}
	var envNotFound Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envNotFound); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envNotFound.Success || envNotFound.Error == nil || envNotFound.Error.Code != "ERR_LOCK_NOT_FOUND" {
		t.Errorf("expected ERR_LOCK_NOT_FOUND, got %v", envNotFound.Error)
	}
}

func setupSleepMemEnv(mem *env.MemEnv) {
	mem.CurrentTime = time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	mem.Files["/proc/meminfo"] = []byte("MemTotal: 4000000 kB\nDirty: 0 kB\n")
	mem.Files["/sys/class/rtc/rtc0/wakealarm"] = []byte("1700000000\n")
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}
}

func TestSleepBlockedByActiveLockExit2(t *testing.T) {
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Acquire lock
	runner.Run(context.Background(), []string{"inhibit", "--reason", "Active upgrade"})
	stdout.Reset()
	stderr.Reset()

	// Attempt sleep without force -> should exit 2
	code := runner.Run(context.Background(), []string{"sleep"})
	if code != ExitInhibited {
		t.Fatalf("expected exit code %d (ExitInhibited), got %d", ExitInhibited, code)
	}
	if !strings.Contains(stderr.String(), "INHIBITED (code 2)") {
		t.Errorf("expected INHIBITED in stderr, got: %s", stderr.String())
	}

	// Attempt sleep with JSON -> should exit 2 with typed error
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--json"})
	if code != ExitInhibited {
		t.Fatalf("expected exit code %d (ExitInhibited), got %d", ExitInhibited, code)
	}
	var envInh Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envInh); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envInh.Success || envInh.Error == nil || envInh.Error.Code != "ERR_INHIBITED" {
		t.Errorf("expected ERR_INHIBITED, got %v", envInh.Error)
	}

	// Attempt sleep with --force -> should override and succeed
	setupSleepMemEnv(mem)
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--force"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d with --force, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
}

func TestSleepBlockedByDaytimeWindowExit2(t *testing.T) {
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 14, 0, 0, 0, time.UTC)
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Attempt sleep during daytime window -> should exit 2
	code := runner.Run(context.Background(), []string{"sleep"})
	if code != ExitInhibited {
		t.Fatalf("expected exit code %d, got %d", ExitInhibited, code)
	}
	if !strings.Contains(stderr.String(), "daytime") {
		t.Errorf("expected mention of daytime in stderr: %s", stderr.String())
	}

	// With JSON
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--json"})
	if code != ExitInhibited {
		t.Fatalf("expected exit code %d, got %d", ExitInhibited, code)
	}
	var envDay Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envDay); err != nil {
		t.Fatalf("failed to parse envelope: %v", err)
	}
	if envDay.Success || envDay.Error == nil || envDay.Error.Code != "ERR_DAYTIME_PROTECTION" {
		t.Errorf("expected ERR_DAYTIME_PROTECTION, got %v", envDay.Error)
	}

	// With --force
	setupSleepMemEnv(mem)
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--force"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d with --force, got %d. stderr: %s", ExitOK, code, stderr.String())
	}
}

const testValidManifestYAML = `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
teardown:
  profile_order: ["apps"]
`

const testUnsafeBufferYAML = `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "23:00"
  wake_at: "23:01"
teardown:
  profile_order: ["apps"]
`

func TestRunnerConfigValidationFailureExitCode3(t *testing.T) {
	mem := env.NewMemEnv()
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testUnsafeBufferYAML), 0644)

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "--config", "/etc/arkon/arkon.yaml"})

	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3), got %d (stderr: %s)", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on text error, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "180") {
		t.Fatalf("expected stderr to mention 180s safety buffer, got: %s", stderr.String())
	}
}

func TestRunnerConfigValidationFailureJSON(t *testing.T) {
	mem := env.NewMemEnv()
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testUnsafeBufferYAML), 0644)

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "--config", "/etc/arkon/arkon.yaml", "--json"})

	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3), got %d (stdout: %s)", code, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envelope Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse json envelope: %v (raw: %s)", err, stdout.String())
	}
	if envelope.Success {
		t.Fatal("expected success: false")
	}
	if envelope.Error == nil {
		t.Fatal("expected non-nil error")
	}
	if envelope.Error.Code != "ERR_MANIFEST_VALIDATION" {
		t.Fatalf("expected ERR_MANIFEST_VALIDATION, got %q", envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "180") {
		t.Fatalf("expected error message to mention 180s buffer, got: %s", envelope.Error.Message)
	}
}

func TestRunnerConfigNotFoundExitCode3(t *testing.T) {
	mem := env.NewMemEnv()
	runner, _, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"apply", "--config", "/nonexistent.yaml"})
	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3), got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") && !strings.Contains(stderr.String(), "nonexistent") {
		t.Fatalf("expected error mentioning missing file, got: %s", stderr.String())
	}
}

func TestRunnerDiscoveredConfigValidationFailure(t *testing.T) {
	mem := env.NewMemEnv()
	_ = mem.WriteFile("./arkon.yaml", []byte(testUnsafeBufferYAML), 0644)

	runner, _, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply"})

	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3) on discovered invalid config, got %d", code)
	}
	if !strings.Contains(stderr.String(), "180") {
		t.Fatalf("expected stderr to mention 180s buffer, got: %s", stderr.String())
	}
}

func TestRunnerApplyNonRootRequiresRootExitCode4(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000 // Non-root
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testValidManifestYAML), 0644)

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "-c", "/etc/arkon/arkon.yaml"})

	if code != ExitInsufficientPrivilege {
		t.Fatalf("expected ExitInsufficientPrivilege (4), got %d (stderr: %s)", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout on permission error, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Root privileges required") {
		t.Errorf("expected stderr to mention root privileges, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "sudo arkon apply") {
		t.Errorf("expected stderr to mention sudo, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "arkon apply --dry-run") {
		t.Errorf("expected stderr to mention dry-run, got: %s", stderr.String())
	}

	// JSON mode
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"apply", "-c", "/etc/arkon/arkon.yaml", "--json"})
	if code != ExitInsufficientPrivilege {
		t.Fatalf("expected ExitInsufficientPrivilege (4) in JSON mode, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr in JSON mode, got: %s", stderr.String())
	}

	var envIns Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envIns); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if envIns.Success {
		t.Fatal("expected success: false")
	}
	if envIns.Error == nil || envIns.Error.Code != "ERR_INSUFFICIENT_PERMISSIONS" {
		t.Fatalf("expected ERR_INSUFFICIENT_PERMISSIONS, got %v", envIns.Error)
	}
}

func TestRunnerApplyDryRunNonRootSuccess(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 1000 // Non-root
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testValidManifestYAML), 0644)

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "--dry-run", "-c", "/etc/arkon/arkon.yaml"})

	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d (stderr: %s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "● ARKON SYSTEMD COMPILER") {
		t.Errorf("expected left-rail header in stdout, got: %s", out)
	}
	if !strings.Contains(out, "✔ [1/4] Manifest parsed:") {
		t.Errorf("expected step 1 check in stdout, got: %s", out)
	}
	if !strings.Contains(out, "✔ [2/4] Compiled units:") {
		t.Errorf("expected step 2 check in stdout, got: %s", out)
	}
	if !strings.Contains(out, "✔ [3/4] Verified syntax: systemd-analyze verify") {
		t.Errorf("expected step 3 check in stdout, got: %s", out)
	}
	if !strings.Contains(out, "--- arkon-sleep.service ---") {
		t.Errorf("expected service unit section in stdout, got: %s", out)
	}
	if !strings.Contains(out, "--- arkon-sleep.timer ---") {
		t.Errorf("expected timer unit section in stdout, got: %s", out)
	}
	if !strings.Contains(out, "DRY RUN COMPLETE: Units validated cleanly. No changes written to disk.") {
		t.Errorf("expected dry run complete message, got: %s", out)
	}

	// Verify no unit files written to destination
	if _, exists := mem.Files["/etc/systemd/system/arkon-sleep.service"]; exists {
		t.Fatalf("expected no destination files written during dry run")
	}

	// JSON mode with dry-run
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"apply", "--dry-run", "-c", "/etc/arkon/arkon.yaml", "--json"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0) in JSON mode, got %d", code)
	}

	var envDry Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envDry); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if !envDry.Success {
		t.Fatal("expected success: true")
	}

	dataBytes, _ := json.Marshal(envDry.Data)
	var applyData systemd.ApplyData
	if err := json.Unmarshal(dataBytes, &applyData); err != nil {
		t.Fatalf("failed to parse ApplyData: %v", err)
	}

	if !applyData.DryRun {
		t.Errorf("expected DryRun = true")
	}
	if !applyData.Validated {
		t.Errorf("expected Validated = true")
	}
	if applyData.SystemdReload {
		t.Errorf("expected SystemdReload = false")
	}
	if applyData.TimerEnabled {
		t.Errorf("expected TimerEnabled = false")
	}
	if applyData.UnitContents == nil || applyData.UnitContents["arkon-sleep.service"] == "" {
		t.Errorf("expected unit contents in ApplyData")
	}
}

func TestRunnerApplyRootSuccess(t *testing.T) {
	mem := env.NewMemEnv()
	mem.EUID = 0
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testValidManifestYAML), 0644)

	var commands []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "-c", "/etc/arkon/arkon.yaml"})

	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d (stderr: %s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "✔ [4/4] Activated units: systemctl daemon-reload && systemctl enable --now arkon-sleep.timer") {
		t.Errorf("expected activation step in stdout, got: %s", out)
	}
	if !strings.Contains(out, "Installation successful!") {
		t.Errorf("expected installation successful in stdout, got: %s", out)
	}

	// Verify unit files committed
	if _, exists := mem.Files["/etc/systemd/system/arkon-sleep.service"]; !exists {
		t.Errorf("expected arkon-sleep.service to be written to /etc/systemd/system/")
	}
	if _, exists := mem.Files["/etc/systemd/system/arkon-sleep.timer"]; !exists {
		t.Errorf("expected arkon-sleep.timer to be written to /etc/systemd/system/")
	}

	// Verify JSON output
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"apply", "-c", "/etc/arkon/arkon.yaml", "--json"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK in JSON mode, got %d", code)
	}

	var envSuccess Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envSuccess); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if !envSuccess.Success {
		t.Fatal("expected success: true")
	}

	dataBytes, _ := json.Marshal(envSuccess.Data)
	var applyData systemd.ApplyData
	if err := json.Unmarshal(dataBytes, &applyData); err != nil {
		t.Fatalf("failed to parse ApplyData: %v", err)
	}
	if applyData.DryRun {
		t.Errorf("expected DryRun = false")
	}
	if !applyData.SystemdReload {
		t.Errorf("expected SystemdReload = true")
	}
	if !applyData.TimerEnabled {
		t.Errorf("expected TimerEnabled = true")
	}
}

func TestRunnerApplyMissingManifestExitCode3(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)

	code := runner.Run(context.Background(), []string{"apply"})
	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3), got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no manifest found") {
		t.Fatalf("expected stderr to mention no manifest found, got: %s", stderr.String())
	}

	// JSON mode
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"apply", "--json"})
	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3) in JSON mode, got %d", code)
	}

	var envErr Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envErr); err != nil {
		t.Fatalf("failed to parse JSON envelope: %v", err)
	}
	if envErr.Success {
		t.Fatal("expected success: false")
	}
	if envErr.Error == nil || envErr.Error.Code != "ERR_MANIFEST_LOAD" {
		t.Fatalf("expected ERR_MANIFEST_LOAD, got: %v", envErr.Error)
	}
}

func TestRunnerApplyVerificationErrorExitCode3(t *testing.T) {
	mem := env.NewMemEnv()
	_ = mem.WriteFile("/etc/arkon/arkon.yaml", []byte(testValidManifestYAML), 0644)
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemd-analyze" {
			return []byte("bad unit file setting: syntax error"), errors.New("exit status 1")
		}
		return nil, errors.New("command not mocked")
	}

	runner, stdout, stderr := newTestRunner(mem)
	code := runner.Run(context.Background(), []string{"apply", "--dry-run", "-c", "/etc/arkon/arkon.yaml"})
	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3), got %d (stderr: %s)", code, stderr.String())
	}

	// JSON mode
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"apply", "--dry-run", "-c", "/etc/arkon/arkon.yaml", "--json"})
	if code != ExitValidationFailure {
		t.Fatalf("expected ExitValidationFailure (3) in JSON mode, got %d", code)
	}

	var envErr Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envErr); err != nil {
		t.Fatalf("failed to decode JSON envelope: %v", err)
	}
	if envErr.Success {
		t.Fatal("expected success: false")
	}
	if envErr.Error == nil || envErr.Error.Code != "ERR_COMPILATION_FAILED" {
		t.Fatalf("expected ERR_COMPILATION_FAILED, got: %v", envErr.Error)
	}
}

func TestSleepDryRunText(t *testing.T) {
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"sleep", "--dry-run"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE (DRY RUN)") {
		t.Errorf("expected dry run header in stdout: %s", out)
	}
	if !strings.Contains(out, "Invariants confirmed") {
		t.Errorf("expected invariants confirmed in stdout: %s", out)
	}
	if !strings.Contains(out, "Workload Teardown (simulated)") {
		t.Errorf("expected simulated teardown in stdout: %s", out)
	}
	if !strings.Contains(out, "Hardware Wakealarm (simulated)") {
		t.Errorf("expected simulated wakealarm in stdout: %s", out)
	}
	if !strings.Contains(out, "Dry run complete") {
		t.Errorf("expected dry run complete in stdout: %s", out)
	}
}

func TestSleepDryRunJSON(t *testing.T) {
	mem := env.NewMemEnv()
	mem.CurrentTime = time.Date(2026, 10, 19, 23, 30, 0, 0, time.UTC)
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"sleep", "--dry-run", "--json"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envData Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envData); err != nil {
		t.Fatalf("failed to parse json envelope: %v (raw: %s)", err, stdout.String())
	}
	if !envData.Success {
		t.Fatal("expected success: true")
	}
	if envData.Error != nil {
		t.Fatalf("expected error: nil, got %v", envData.Error)
	}

	dataMap, ok := envData.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map in data, got %T", envData.Data)
	}
	if dataMap["dry_run"] != true {
		t.Errorf("expected dry_run: true, got %v", dataMap["dry_run"])
	}
	if dataMap["status"] != "completed" {
		t.Errorf("expected status: completed, got %v", dataMap["status"])
	}
	if dataMap["wakealarm_armed"] != true {
		t.Errorf("expected wakealarm_armed: true, got %v", dataMap["wakealarm_armed"])
	}
}

func TestSleepFullPipelineSuccess(t *testing.T) {
	mem := env.NewMemEnv()
	setupSleepMemEnv(mem)

	var executedCmds []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executedCmds = append(executedCmds, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"sleep"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "● ARKON TEARDOWN & HARDWARE SLEEP PIPELINE") {
		t.Errorf("expected sleep pipeline header in stdout: %s", out)
	}
	if !strings.Contains(out, "Teardown complete") {
		t.Errorf("expected teardown complete in stdout: %s", out)
	}

	foundPoweroff := false
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "systemctl poweroff") {
			foundPoweroff = true
			break
		}
	}
	if !foundPoweroff {
		t.Errorf("expected systemctl poweroff in executed commands: %v", executedCmds)
	}
}

func TestSleepFullPipelineJSON(t *testing.T) {
	mem := env.NewMemEnv()
	setupSleepMemEnv(mem)

	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"sleep", "--json"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got: %s", stderr.String())
	}

	var envelope Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode json envelope: %v", err)
	}
	if !envelope.Success {
		t.Fatal("expected success: true")
	}

	dataMap := envelope.Data.(map[string]any)
	if dataMap["disk_synced"] != true {
		t.Errorf("expected disk_synced: true, got %v", dataMap["disk_synced"])
	}
	if dataMap["wakealarm_armed"] != true {
		t.Errorf("expected wakealarm_armed: true, got %v", dataMap["wakealarm_armed"])
	}
	if dataMap["power_action"] != "poweroff" {
		t.Errorf("expected power_action: poweroff, got %v", dataMap["power_action"])
	}
}

func TestSleepWithManifestConfig(t *testing.T) {
	mem := env.NewMemEnv()
	setupSleepMemEnv(mem)

	manifestYAML := `
version: "1"
host:
  timezone: "UTC"
  power_action: "suspend"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
teardown:
  profile_order: ["apps", "infra"]
hooks:
  pre_sleep:
    - name: "backup-state"
      command: "echo backing up"
`
	_ = mem.WriteFile("/tmp/arkon.yaml", []byte(manifestYAML), 0644)

	var executedCmds []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executedCmds = append(executedCmds, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	runner, _, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"sleep", "--config", "/tmp/arkon.yaml"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
	}

	foundHook := false
	foundSuspend := false
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "echo backing up") {
			foundHook = true
		}
		if strings.Contains(cmd, "systemctl suspend") {
			foundSuspend = true
		}
	}
	if !foundHook {
		t.Errorf("expected pre-sleep hook command in executed commands: %v", executedCmds)
	}
	if !foundSuspend {
		t.Errorf("expected systemctl suspend in executed commands: %v", executedCmds)
	}
}

func TestSleepFailureExit1(t *testing.T) {
	mem := env.NewMemEnv()
	setupSleepMemEnv(mem)

	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" {
			return []byte("permission denied"), errors.New("exit status 1")
		}
		return []byte("ok"), nil
	}

	t.Run("text failure", func(t *testing.T) {
		runner, _, stderr := newTestRunner(mem)
		runner.StateDir = t.TempDir()

		code := runner.Run(context.Background(), []string{"sleep"})
		if code != ExitGeneralError {
			t.Fatalf("expected ExitGeneralError (1), got %d", code)
		}
		if !strings.Contains(stderr.String(), "ERROR (code 1)") {
			t.Errorf("expected ERROR (code 1) in stderr: %s", stderr.String())
		}
	})

	t.Run("json failure", func(t *testing.T) {
		runner, stdout, _ := newTestRunner(mem)
		runner.StateDir = t.TempDir()

		code := runner.Run(context.Background(), []string{"sleep", "--json"})
		if code != ExitGeneralError {
			t.Fatalf("expected ExitGeneralError (1), got %d", code)
		}
		var envErr Envelope
		if err := json.Unmarshal(stdout.Bytes(), &envErr); err != nil {
			t.Fatalf("failed to decode envelope: %v", err)
		}
		if envErr.Success {
			t.Fatal("expected success: false")
		}
		if envErr.Error == nil || envErr.Error.Code != "ERR_SLEEP_FAILED" {
			t.Errorf("expected ERR_SLEEP_FAILED, got %v", envErr.Error)
		}
	})
}

func TestWakeCommand(t *testing.T) {
	mem := env.NewMemEnv()
	manifestYAML := `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
teardown:
  profile_order: ["apps"]
hooks:
  post_wake:
    - name: "notify-wake"
      command: "echo host is awake"
`
	mem.Files["/etc/arkon/arkon.yaml"] = []byte(manifestYAML)

	var executed []string
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executed = append(executed, name+" "+strings.Join(args, " "))
		return []byte("ok"), nil
	}

	t.Run("human output", func(t *testing.T) {
		runner, stdout, stderr := newTestRunner(mem)
		runner.StateDir = t.TempDir()

		code := runner.Run(context.Background(), []string{"wake"})
		if code != ExitOK {
			t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Post-wake lifecycle sequence completed") {
			t.Errorf("expected completion message in stdout: %s", stdout.String())
		}
	})

	t.Run("json output", func(t *testing.T) {
		runner, stdout, stderr := newTestRunner(mem)
		runner.StateDir = t.TempDir()

		code := runner.Run(context.Background(), []string{"wake", "--json"})
		if code != ExitOK {
			t.Fatalf("expected ExitOK (0), got %d. stderr: %s", code, stderr.String())
		}
		var envWake Envelope
		if err := json.Unmarshal(stdout.Bytes(), &envWake); err != nil {
			t.Fatalf("failed to decode envelope: %v", err)
		}
		if !envWake.Success {
			t.Fatalf("expected success: true, got %v", envWake)
		}
	})

	t.Run("failure on hook error", func(t *testing.T) {
		memFail := env.NewMemEnv()
		memFail.Files["/etc/arkon/arkon.yaml"] = []byte(manifestYAML)
		memFail.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("command failed")
		}

		runner, _, stderr := newTestRunner(memFail)
		runner.StateDir = t.TempDir()

		code := runner.Run(context.Background(), []string{"wake"})
		if code != ExitGeneralError {
			t.Fatalf("expected ExitGeneralError (1), got %d", code)
		}
		if !strings.Contains(stderr.String(), "post-wake hooks failed") {
			t.Errorf("expected error message in stderr: %s", stderr.String())
		}
	})
}

func TestInhibitDefaultDurationFromManifest(t *testing.T) {
	mem := env.NewMemEnv()
	fixedTime := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	mem.CurrentTime = fixedTime

	manifestYAML := `
version: "1"
host:
  timezone: "UTC"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
teardown:
  profile_order: ["apps"]
inhibition:
  default_duration: "4h"
`
	mem.Files["/etc/arkon/arkon.yaml"] = []byte(manifestYAML)

	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	code := runner.Run(context.Background(), []string{"inhibit", "--json"})
	if code != ExitOK {
		t.Fatalf("expected ExitOK, got %d. stderr: %s", code, stderr.String())
	}

	var envLock Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envLock); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}
	if !envLock.Success {
		t.Fatalf("expected success: true, got %v", envLock)
	}

	lockMap, ok := envLock.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for lock, got %T", envLock.Data)
	}

	// Lock expires_at should be 4 hours ahead (16:00 UTC)
	expiresAtStr, ok := lockMap["expires_at"].(string)
	if !ok {
		t.Fatalf("expected string expires_at, got %v", lockMap["expires_at"])
	}
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		t.Fatalf("failed to parse expires_at %q: %v", expiresAtStr, err)
	}

	expectedExpires := fixedTime.Add(4 * time.Hour)
	if !expiresAt.Equal(expectedExpires) {
		t.Fatalf("expected expiration at %v (4h), got %v", expectedExpires, expiresAt)
	}
}
