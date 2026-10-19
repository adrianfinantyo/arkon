package state

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

// withFileLock acquires an exclusive flock on the lock file, invokes the callback, and releases the lock.
func (m *Manager) withFileLock(fn func() error) error {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	lockFile, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file %s: %w", m.lockPath, err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire flock on %s: %w", m.lockPath, err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	return fn()
}

// saveStateAtomic writes the state object to a temporary file in m.dir and renames it to state.json.
func (m *Manager) saveStateAtomic(state *State) error {
	if state.Locks == nil {
		state.Locks = []InhibitionLock{}
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize state: %w", err)
	}
	data = append(data, '\n')

	tempFile, err := os.CreateTemp(m.dir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}
	tempPath := tempFile.Name()

	cleanup := func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}

	if _, err := tempFile.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("failed to write state data: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("failed to sync state file to disk: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to close temporary state file: %w", err)
	}

	if err := os.Rename(tempPath, m.statePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to replace state file %s: %w", m.statePath, err)
	}

	return nil
}
