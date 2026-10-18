package env

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"sync"
	"testing/fstest"
	"time"
)

// MemEnv implements HostEnvironment in memory for deterministic unit and integration testing.
type MemEnv struct {
	mu          sync.RWMutex
	Files       map[string][]byte
	EnvVars     map[string]string
	CurrentTime time.Time
	EUID        int
	SyncCount   int
	ExecHandler  func(ctx context.Context, name string, args ...string) ([]byte, error)
	WriteHandler func(path string, data []byte, perm os.FileMode) error
	ReadHandler  func(path string) ([]byte, error)
}

// NewMemEnv creates an in-memory test environment.
func NewMemEnv() *MemEnv {
	return &MemEnv{
		Files:       make(map[string][]byte),
		EnvVars:     make(map[string]string),
		CurrentTime: time.Now(),
		EUID:        0,
	}
}

func (m *MemEnv) FS() fs.FS {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mapFS := fstest.MapFS{}
	for k, v := range m.Files {
		clean := bytes.TrimLeft([]byte(k), "/")
		mapFS[string(clean)] = &fstest.MapFile{
			Data: v,
			Mode: 0644,
		}
	}
	return mapFS
}

func (m *MemEnv) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.WriteHandler != nil {
		return m.WriteHandler(path, data, perm)
	}

	cp := make([]byte, len(data))
	copy(cp, data)
	m.Files[path] = cp
	return nil
}

func (m *MemEnv) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.ReadHandler != nil {
		return m.ReadHandler(path)
	}

	data, exists := m.Files[path]
	if !exists {
		return nil, os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (m *MemEnv) Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.mu.RLock()
	handler := m.ExecHandler
	m.mu.RUnlock()

	if handler != nil {
		return handler(ctx, name, args...)
	}
	return nil, errors.New("command not mocked")
}

func (m *MemEnv) SyncFS() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SyncCount++
	return nil
}

func (m *MemEnv) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CurrentTime
}

func (m *MemEnv) Geteuid() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.EUID
}

func (m *MemEnv) LookupEnv(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.EnvVars[key]
	return val, ok
}
