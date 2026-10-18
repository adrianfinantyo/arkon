package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adrianfinantyo/arkon/internal/env"
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

func TestPlaceholderSubcommands(t *testing.T) {
	commands := []string{"apply", "status"}

	for _, cmd := range commands {
		t.Run(cmd+" text", func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), []string{cmd})
			if code != ExitGeneralError {
				t.Fatalf("expected exit code %d, got %d", ExitGeneralError, code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got: %s", stdout.String())
			}
			if !strings.Contains(stderr.String(), cmd) {
				t.Fatalf("expected error mentioning command %s, got: %s", cmd, stderr.String())
			}
		})

		t.Run(cmd+" json", func(t *testing.T) {
			mem := env.NewMemEnv()
			runner, stdout, stderr := newTestRunner(mem)

			code := runner.Run(context.Background(), []string{cmd, "--json"})
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
			if envelope.Error == nil || envelope.Error.Code != "ERR_NOT_IMPLEMENTED" {
				t.Fatalf("expected ERR_NOT_IMPLEMENTED, got: %v", envelope.Error)
			}
		})
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

func TestSleepBlockedByActiveLockExit2(t *testing.T) {
	mem := env.NewMemEnv()
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
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--force"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d with --force, got %d", ExitOK, code)
	}
}

func TestSleepBlockedByDaytimeWindowExit2(t *testing.T) {
	mem := env.NewMemEnv()
	runner, stdout, stderr := newTestRunner(mem)
	runner.StateDir = t.TempDir()

	// Attempt sleep with simulated daytime window -> should exit 2
	code := runner.Run(context.Background(), []string{"sleep", "--simulate-daytime"})
	if code != ExitInhibited {
		t.Fatalf("expected exit code %d, got %d", ExitInhibited, code)
	}
	if !strings.Contains(stderr.String(), "daytime") {
		t.Errorf("expected mention of daytime in stderr: %s", stderr.String())
	}

	// With JSON
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--simulate-daytime", "--json"})
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
	stdout.Reset()
	stderr.Reset()
	code = runner.Run(context.Background(), []string{"sleep", "--simulate-daytime", "--force"})
	if code != ExitOK {
		t.Fatalf("expected exit code %d with --force, got %d", ExitOK, code)
	}
}


