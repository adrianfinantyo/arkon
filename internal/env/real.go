package env

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// RealEnv implements HostEnvironment using the real host operating system.
type RealEnv struct{}

// NewRealEnv creates a real host environment.
func NewRealEnv() *RealEnv {
	return &RealEnv{}
}

func (e *RealEnv) FS() fs.FS {
	return os.DirFS("/")
}

func (e *RealEnv) WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func (e *RealEnv) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (e *RealEnv) RemoveFile(path string) error {
	return os.Remove(path)
}

func (e *RealEnv) RenameFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (e *RealEnv) Symlink(target, linkPath string) error {
	dir := filepath.Dir(linkPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	_ = os.Remove(linkPath)
	return os.Symlink(target, linkPath)
}

func (e *RealEnv) Readlink(linkPath string) (string, error) {
	return os.Readlink(linkPath)
}

func (e *RealEnv) Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (e *RealEnv) SyncFS() error {
	syscall.Sync()
	return nil
}

func (e *RealEnv) Now() time.Time {
	return time.Now()
}

func (e *RealEnv) Geteuid() int {
	return os.Geteuid()
}

func (e *RealEnv) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
