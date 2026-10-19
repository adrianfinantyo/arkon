package env

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRealEnvOperations(t *testing.T) {
	env := NewRealEnv()

	// Verify Now() returns reasonable current time
	if env.Now().Before(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected past time: %v", env.Now())
	}

	// Verify Geteuid returns os.Geteuid()
	if env.Geteuid() != os.Geteuid() {
		t.Fatalf("expected euid %d, got %d", os.Geteuid(), env.Geteuid())
	}

	// Verify LookupEnv
	os.Setenv("ARKON_TEST_VAR", "test_value")
	defer os.Unsetenv("ARKON_TEST_VAR")
	val, ok := env.LookupEnv("ARKON_TEST_VAR")
	if !ok || val != "test_value" {
		t.Fatalf("expected test_value, got %q (found: %v)", val, ok)
	}

	// Verify WriteFile and ReadFile
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "sample.txt")
	testData := []byte("hello arkon")

	if err := env.WriteFile(testPath, testData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	readBack, err := env.ReadFile(testPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readBack) != string(testData) {
		t.Fatalf("expected %q, got %q", string(testData), string(readBack))
	}

	// Verify RenameFile
	renamedPath := filepath.Join(tmpDir, "renamed.txt")
	if err := env.WriteFile(testPath, testData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := env.RenameFile(testPath, renamedPath); err != nil {
		t.Fatalf("RenameFile failed: %v", err)
	}
	if _, err := env.ReadFile(testPath); err == nil {
		t.Fatal("expected old path to not exist after RenameFile")
	}
	renamedData, err := env.ReadFile(renamedPath)
	if err != nil || string(renamedData) != string(testData) {
		t.Fatalf("expected renamed file data %q, got %q (err: %v)", string(testData), string(renamedData), err)
	}

	// Verify RemoveFile
	if err := env.RemoveFile(renamedPath); err != nil {
		t.Fatalf("RemoveFile failed: %v", err)
	}
	if _, err := env.ReadFile(renamedPath); err == nil {
		t.Fatal("expected error reading removed file in RealEnv")
	}

	// Verify Exec
	out, err := env.Exec(context.Background(), "echo", "test-exec")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if string(out) != "test-exec\n" {
		t.Fatalf("unexpected Exec output: %q", string(out))
	}

	// Verify SyncFS
	if err := env.SyncFS(); err != nil {
		t.Fatalf("SyncFS failed: %v", err)
	}

	// Verify Symlink and Readlink
	linkTarget := filepath.Join(tmpDir, "target.txt")
	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	if err := env.WriteFile(linkTarget, testData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := env.Symlink(linkTarget, symlinkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}
	target, err := env.Readlink(symlinkPath)
	if err != nil || target != linkTarget {
		t.Fatalf("expected target %q, got %q (err: %v)", linkTarget, target, err)
	}
	symlinkData, err := env.ReadFile(symlinkPath)
	if err != nil || string(symlinkData) != string(testData) {
		t.Fatalf("expected %q, got %q (err: %v)", string(testData), string(symlinkData), err)
	}
}

func TestMemEnvOperations(t *testing.T) {
	fixedTime := time.Date(2026, 10, 19, 7, 0, 0, 0, time.UTC)
	mem := NewMemEnv()
	mem.CurrentTime = fixedTime
	mem.EUID = 1000
	mem.EnvVars["CUSTOM_KEY"] = "custom_val"

	if mem.Now() != fixedTime {
		t.Fatalf("expected fixed time %v, got %v", fixedTime, mem.Now())
	}

	if mem.Geteuid() != 1000 {
		t.Fatalf("expected euid 1000, got %d", mem.Geteuid())
	}

	val, ok := mem.LookupEnv("CUSTOM_KEY")
	if !ok || val != "custom_val" {
		t.Fatalf("expected custom_val, got %q", val)
	}

	// Test memory file writes and reads
	path := "/etc/arkon/arkon.yaml"
	content := []byte("version: '1'")
	if err := mem.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("MemEnv WriteFile failed: %v", err)
	}

	readBack, err := mem.ReadFile(path)
	if err != nil {
		t.Fatalf("MemEnv ReadFile failed: %v", err)
	}
	if string(readBack) != string(content) {
		t.Fatalf("expected %q, got %q", string(content), string(readBack))
	}

	// Verify non-existent file error
	if _, err := mem.ReadFile("/nonexistent"); err == nil {
		t.Fatal("expected error reading nonexistent file in MemEnv")
	}

	// Verify RemoveFile
	if err := mem.RemoveFile(path); err != nil {
		t.Fatalf("MemEnv RemoveFile failed: %v", err)
	}
	if _, err := mem.ReadFile(path); err == nil {
		t.Fatal("expected error reading removed file in MemEnv")
	}
	if err := mem.RemoveFile("/nonexistent"); err == nil {
		t.Fatal("expected error removing nonexistent file in MemEnv")
	}

	// Verify RenameFile
	if err := mem.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("MemEnv WriteFile failed: %v", err)
	}
	renamedMemPath := "/etc/arkon/renamed.yaml"
	if err := mem.RenameFile(path, renamedMemPath); err != nil {
		t.Fatalf("MemEnv RenameFile failed: %v", err)
	}
	if _, err := mem.ReadFile(path); err == nil {
		t.Fatal("expected old path to not exist after RenameFile in MemEnv")
	}
	renamedMemData, err := mem.ReadFile(renamedMemPath)
	if err != nil || string(renamedMemData) != string(content) {
		t.Fatalf("expected %q, got %q (err: %v)", string(content), string(renamedMemData), err)
	}
	if err := mem.RenameFile("/nonexistent", "/other"); err == nil {
		t.Fatal("expected error renaming nonexistent file in MemEnv")
	}

	// Test exec handler
	mem.ExecHandler = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("mock-output"), nil
	}
	out, err := mem.Exec(context.Background(), "docker", "ps")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if string(out) != "mock-output" {
		t.Fatalf("unexpected output: %q", string(out))
	}

	// Verify sync tracking
	if mem.SyncCount != 0 {
		t.Fatalf("expected 0 sync calls, got %d", mem.SyncCount)
	}
	if err := mem.SyncFS(); err != nil {
		t.Fatalf("SyncFS failed: %v", err)
	}
	if mem.SyncCount != 1 {
		t.Fatalf("expected 1 sync call, got %d", mem.SyncCount)
	}

	// Verify WriteHandler and ReadHandler customization
	customIntercepted := false
	mem.WriteHandler = func(path string, data []byte, perm os.FileMode) error {
		customIntercepted = true
		mem.Files[path] = []byte("intercepted:" + string(data))
		return nil
	}
	if err := mem.WriteFile("/custom/file", []byte("payload"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if !customIntercepted {
		t.Fatal("expected WriteHandler to be called")
	}

	mem.ReadHandler = func(path string) ([]byte, error) {
		return []byte("custom-read:" + path), nil
	}
	customRead, err := mem.ReadFile("/any/path")
	if err != nil {
		t.Fatalf("ReadFile with ReadHandler failed: %v", err)
	}
	if string(customRead) != "custom-read:/any/path" {
		t.Fatalf("unexpected custom read: %q", string(customRead))
	}
}

func TestMemEnvSymlinks(t *testing.T) {
	mem := NewMemEnv()
	realPath := "/etc/arkon/real.yaml"
	linkPath := "/etc/arkon/arkon.yaml"
	content := []byte("version: '1'")

	if err := mem.WriteFile(realPath, content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := mem.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	target, err := mem.Readlink(linkPath)
	if err != nil || target != realPath {
		t.Fatalf("expected %q, got %q (err: %v)", realPath, target, err)
	}

	readViaLink, err := mem.ReadFile(linkPath)
	if err != nil || string(readViaLink) != string(content) {
		t.Fatalf("expected %q, got %q (err: %v)", string(content), string(readViaLink), err)
	}

	if err := mem.RemoveFile(linkPath); err != nil {
		t.Fatalf("RemoveFile on symlink failed: %v", err)
	}
	if _, err := mem.Readlink(linkPath); err == nil {
		t.Fatal("expected error reading removed symlink")
	}
	// Target should still exist
	if _, err := mem.ReadFile(realPath); err != nil {
		t.Fatalf("expected real target to persist: %v", err)
	}
}
