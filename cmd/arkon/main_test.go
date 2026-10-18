package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBinaryCLI(t *testing.T) {
	// Build temporary test binary
	tmpBin := t.TempDir() + "/arkon"
	buildCmd := exec.Command("go", "build", "-o", tmpBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v (output: %s)", err, string(out))
	}

	t.Run("version flag", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "version")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("version failed: %v", err)
		}
		if !strings.Contains(string(out), "arkon dev") {
			t.Fatalf("expected arkon dev in output: %s", string(out))
		}
	})

	t.Run("version json flag", func(t *testing.T) {
		cmd := exec.Command(tmpBin, "version", "--json")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("version --json failed: %v", err)
		}
		if !strings.Contains(string(out), `"success": true`) {
			t.Fatalf("expected success: true in json output: %s", string(out))
		}
	})

	t.Run("help flag", func(t *testing.T) {
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
		if !strings.Contains(string(out), "Unknown command: not-a-command") {
			t.Fatalf("expected unknown command diagnostic: %s", string(out))
		}
	})
}
