package env

import (
	"context"
	"io/fs"
	"os"
	"time"
)

// HostEnvironment defines the operating system seam for Arkon.
// All hardware, filesystem, execution, and time interactions flow through this interface,
// enabling full testability without elevated host privileges.
type HostEnvironment interface {
	FS() fs.FS
	WriteFile(path string, data []byte, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	RemoveFile(path string) error
	RenameFile(oldPath, newPath string) error
	Exec(ctx context.Context, name string, args ...string) ([]byte, error)
	SyncFS() error
	Now() time.Time
	Geteuid() int
	LookupEnv(key string) (string, bool)
}
