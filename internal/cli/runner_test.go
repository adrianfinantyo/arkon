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
	commands := []string{"apply", "status", "sleep", "inhibit"}

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

