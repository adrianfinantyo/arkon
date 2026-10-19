package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryCLI(t *testing.T) {
	// Build temporary test binary
	tmpBin := filepath.Join(t.TempDir(), "arkon")
	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v (output: %s)", err, string(out))
	}

	t.Run("version flag exit 0", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("version failed: %v", err)
		}
		if !strings.Contains(string(out), "arkon dev") {
			t.Fatalf("expected arkon dev in output: %s", string(out))
		}
	})

	t.Run("version json flag exit 0", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "version", "--json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("version --json failed: %v", err)
		}
		if !strings.Contains(string(out), `"success": true`) {
			t.Fatalf("expected success: true in json output: %s", string(out))
		}
	})

	t.Run("help flag exit 0", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "--help")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if !strings.Contains(string(out), "Available Commands:") {
			t.Fatalf("expected usage in help output: %s", string(out))
		}
	})

	t.Run("unknown command exit 1", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "not-a-command")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected error exit code for unknown command")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got: %v", err)
		}
		if !strings.Contains(string(out), "Unknown command: not-a-command") {
			t.Fatalf("expected unknown command diagnostic: %s", string(out))
		}
	})

	t.Run("inhibition lock and daytime protection exit 2", func(t *testing.T) {
		tmpDir := t.TempDir()
		stateDir := filepath.Join(tmpDir, "state")
		env := append(os.Environ(), "ARKON_STATE_DIR="+stateDir)

		// 1. Test daytime protection exit code 2
		allDaytimePath := filepath.Join(tmpDir, "daytime.yaml")
		allDaytimeContent := `version: "1"
host:
  timezone: "UTC"
  rtc_device: "/sys/class/rtc/rtc0"
  power_action: "poweroff"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
teardown:
  timeout_seconds: 10
  profile_order: ["apps"]
inhibition:
  default_duration: "2h"
  daytime_protection: true
  daytime_start: "00:00"
  daytime_end: "23:59"
`
		if err := os.WriteFile(allDaytimePath, []byte(allDaytimeContent), 0644); err != nil {
			t.Fatalf("failed to write daytime manifest: %v", err)
		}

		sleepDaytimeCmd := exec.Command(tmpBin, "sleep", "-c", allDaytimePath)
		sleepDaytimeCmd.Env = env
		outDaytime, errDaytime := sleepDaytimeCmd.CombinedOutput()
		if errDaytime == nil {
			t.Fatal("expected sleep command to fail during daytime")
		}
		exitDaytimeErr, ok := errDaytime.(*exec.ExitError)
		if !ok || exitDaytimeErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2 for daytime window, got: %v (output: %s)", errDaytime, string(outDaytime))
		}
		if !strings.Contains(string(outDaytime), "Refusing sleep during daytime window") {
			t.Fatalf("expected daytime refusal in output: %s", string(outDaytime))
		}

		// 2. Test active inhibition lock exit code 2 (with daytime protection disabled)
		noDaytimePath := filepath.Join(tmpDir, "no_daytime.yaml")
		noDaytimeContent := `version: "1"
host:
  timezone: "UTC"
  rtc_device: "/sys/class/rtc/rtc0"
  power_action: "poweroff"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
teardown:
  timeout_seconds: 10
  profile_order: ["apps"]
inhibition:
  default_duration: "2h"
  daytime_protection: false
`
		if err := os.WriteFile(noDaytimePath, []byte(noDaytimeContent), 0644); err != nil {
			t.Fatalf("failed to write no-daytime manifest: %v", err)
		}

		// Acquire inhibition lock
		inhibitCmd := exec.Command(tmpBin, "inhibit", "--reason", "Binary integration test", "--duration", "1h")
		inhibitCmd.Env = env
		if out, err := inhibitCmd.CombinedOutput(); err != nil {
			t.Fatalf("inhibit command failed: %v (output: %s)", err, string(out))
		}

		// Attempt sleep in human mode
		sleepCmd := exec.Command(tmpBin, "sleep", "-c", noDaytimePath)
		sleepCmd.Env = env
		out, err := sleepCmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected sleep command to exit with error when inhibited by lock")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2, got: %v (output: %s)", err, string(out))
		}
		if !strings.Contains(string(out), "INHIBITED (code 2)") {
			t.Fatalf("expected INHIBITED (code 2) in output: %s", string(out))
		}

		// Attempt sleep in JSON mode
		sleepJSONCmd := exec.Command(tmpBin, "sleep", "-c", noDaytimePath, "--json")
		sleepJSONCmd.Env = env
		outJSON, errJSON := sleepJSONCmd.CombinedOutput()
		if errJSON == nil {
			t.Fatal("expected sleep --json to exit with error code 2")
		}
		exitJSONErr, ok := errJSON.(*exec.ExitError)
		if !ok || exitJSONErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2, got: %v (output: %s)", errJSON, string(outJSON))
		}

		var envelope map[string]any
		if err := json.Unmarshal(outJSON, &envelope); err != nil {
			t.Fatalf("failed to parse JSON envelope: %v (raw: %s)", err, string(outJSON))
		}
		if envelope["success"] != false {
			t.Fatalf("expected success: false, got: %v", envelope["success"])
		}
		errMap, ok := envelope["error"].(map[string]any)
		if !ok || errMap["code"] != "ERR_INHIBITED" {
			t.Fatalf("expected ERR_INHIBITED code, got: %v", envelope["error"])
		}
	})

	t.Run("manifest validation and not found exit 3", func(t *testing.T) {
		// Non-existent manifest path
		cmdNotFound := exec.Command(tmpBin, "apply", "--config", "/nonexistent/arkon.yaml")
		outNotFound, errNotFound := cmdNotFound.CombinedOutput()
		if errNotFound == nil {
			t.Fatal("expected apply with non-existent config to fail")
		}
		exitErr, ok := errNotFound.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 3 {
			t.Fatalf("expected exit code 3 for missing config, got: %v (output: %s)", errNotFound, string(outNotFound))
		}

		// Invalid manifest content
		tmpDir := t.TempDir()
		invalidPath := filepath.Join(tmpDir, "invalid.yaml")
		if err := os.WriteFile(invalidPath, []byte("version: \"99\"\n"), 0644); err != nil {
			t.Fatalf("failed to write invalid manifest: %v", err)
		}

		cmdInvalid := exec.Command(tmpBin, "apply", "--config", invalidPath)
		outInvalid, errInvalid := cmdInvalid.CombinedOutput()
		if errInvalid == nil {
			t.Fatal("expected apply with invalid manifest to fail")
		}
		exitInvalidErr, ok := errInvalid.(*exec.ExitError)
		if !ok || exitInvalidErr.ExitCode() != 3 {
			t.Fatalf("expected exit code 3 for invalid manifest, got: %v (output: %s)", errInvalid, string(outInvalid))
		}

		// Invalid manifest with --json flag
		cmdInvalidJSON := exec.Command(tmpBin, "apply", "--config", invalidPath, "--json")
		outInvalidJSON, errInvalidJSON := cmdInvalidJSON.CombinedOutput()
		if errInvalidJSON == nil {
			t.Fatal("expected apply --json with invalid manifest to fail")
		}
		exitJSONErr, ok := errInvalidJSON.(*exec.ExitError)
		if !ok || exitJSONErr.ExitCode() != 3 {
			t.Fatalf("expected exit code 3 for invalid manifest in JSON mode, got: %v", errInvalidJSON)
		}
		var envJSON map[string]any
		if err := json.Unmarshal(outInvalidJSON, &envJSON); err != nil {
			t.Fatalf("failed to decode json envelope: %v", err)
		}
		if envJSON["success"] != false {
			t.Fatalf("expected success: false in json output")
		}
	})

	t.Run("apply non-root without dry-run exit 4", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("skipping non-root privilege test when running as root")
		}

		tmpDir := t.TempDir()
		validPath := filepath.Join(tmpDir, "arkon.yaml")
		validContent := `version: "1"
host:
  timezone: "UTC"
  rtc_device: "/sys/class/rtc/rtc0"
  power_action: "poweroff"
circadian:
  sleep_at: "23:00"
  wake_at: "07:00"
  days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
teardown:
  timeout_seconds: 10
  profile_order: ["apps"]
inhibition:
  default_duration: "2h"
  daytime_protection: true
`
		if err := os.WriteFile(validPath, []byte(validContent), 0644); err != nil {
			t.Fatalf("failed to write valid manifest: %v", err)
		}

		cmd := exec.Command(tmpBin, "apply", "--config", validPath)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("expected apply without sudo to fail with exit code 4")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 4 {
			t.Fatalf("expected exit code 4, got: %v (output: %s)", err, string(out))
		}
		if !strings.Contains(string(out), "code 4") || !strings.Contains(string(out), "Root privileges required") {
			t.Fatalf("expected code 4 and root required message in output: %s", string(out))
		}

		// Verify --json mode returns exit code 4 and ERR_INSUFFICIENT_PERMISSIONS
		cmdJSON := exec.Command(tmpBin, "apply", "--config", validPath, "--json")
		outJSON, errJSON := cmdJSON.CombinedOutput()
		if errJSON == nil {
			t.Fatal("expected apply --json without sudo to fail with exit code 4")
		}
		exitJSONErr, ok := errJSON.(*exec.ExitError)
		if !ok || exitJSONErr.ExitCode() != 4 {
			t.Fatalf("expected exit code 4 in json mode, got: %v (output: %s)", errJSON, string(outJSON))
		}
		var envJSON map[string]any
		if err := json.Unmarshal(outJSON, &envJSON); err != nil {
			t.Fatalf("failed to decode json envelope: %v", err)
		}
		if envJSON["success"] != false {
			t.Fatalf("expected success: false in json output")
		}
		errMap, ok := envJSON["error"].(map[string]any)
		if !ok || errMap["code"] != "ERR_INSUFFICIENT_PERMISSIONS" {
			t.Fatalf("expected ERR_INSUFFICIENT_PERMISSIONS, got: %v", envJSON["error"])
		}
	})
}
